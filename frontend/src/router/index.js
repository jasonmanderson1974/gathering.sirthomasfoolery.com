import { createRouter, createWebHistory } from "vue-router"
import Landing from "@/views/Landing"
import { setUnauthorizedHandler } from "@/utils/fetch_utils"
import store from "@/store"

const routes = [
  {
    path: "/",
    name: "landing",
    component: Landing,
  },
  {
    path: "/home",
    name: "home",
    component: () => import("@/views/Home.vue"),
    props: true,
  },
  {
    path: "/settings",
    name: "settings",
    component: () => import("@/views/Settings.vue"),
  },
  {
    path: "/members",
    name: "admin",
    component: () => import("@/views/MemberAdmin.vue"),
  },
  {
    path: "/fellowship",
    name: "fellowship",
    component: () => import("@/views/Fellowship.vue"),
  },
  {
    path: "/chronicle",
    name: "chronicle",
    component: () => import("@/views/Chronicle.vue"),
  },
  {
    path: "/e/:eventId",
    name: "event",
    component: () => import("@/views/Event.vue"),
    props: true,
  },
  {
    path: "/e/:eventId/responded",
    name: "responded",
    component: () => import("@/views/Responded.vue"),
    props: true,
  },
  {
    path: "/sign-in",
    name: "sign-in",
    component: () => import("@/views/SignIn.vue"),
  },
  {
    path: "/sign-up",
    name: "sign-up",
    component: () => import("@/views/SignIn.vue"),
    props: { initialIsSignUp: true },
  },
  {
    path: "/auth",
    name: "auth",
    component: () => import("@/views/Auth.vue"),
  },
  {
    path: "/privacy-policy",
    name: "privacy-policy",
    component: () => import("@/views/PrivacyPolicy.vue"),
  },
  {
    // Vue Router 4 dropped the bare `*` wildcard; a catch-all is now a named
    // param with a custom regexp. The trailing `*` is the repeatable modifier,
    // without which a multi-segment path like /a/b would not match.
    path: "/:pathMatch(.*)*",
    name: "404",
    component: () => import("@/views/PageNotFound.vue"),
  },
]

const router = createRouter({
  history: createWebHistory(process.env.BASE_URL),
  routes,
})

// Routes reachable without a session. Everything else requires one, so a route
// added later is gated by default rather than open by default — which is how
// `chronicle` ended up unprotected under the old opt-in list.
const publicRoutes = [
  "landing",
  "sign-in",
  "sign-up",
  "auth",
  "privacy-policy",
  "404",
]

// Only same-origin paths may be redirected to, so a crafted
// ?redirect=https://evil.example link can't turn our login into an open
// redirect. A leading "//" is protocol-relative, i.e. off-site.
export const isSafeRedirect = (path) =>
  typeof path === "string" && path.startsWith("/") && !path.startsWith("//")

router.beforeEach(async (to, from, next) => {
  // A20: this used to await GET /auth/status on EVERY navigation, duplicating
  // state the store already holds. Consult the store first and only fetch when
  // it's empty — a cold load, or after signing out.
  let authUser = store.state.authUser
  if (!authUser) {
    try {
      authUser = await store.dispatch("refreshAuthUser")
    } catch {
      authUser = null
    }
  }

  if (!authUser) {
    if (publicRoutes.includes(to.name)) return next()
    // Remember where they were headed so a shared /e/:id link survives login.
    return next({ name: "sign-in", query: { redirect: to.fullPath } })
  }

  // Signed in: keep them off the sign-in screens, but honour a pending
  // redirect rather than dropping it (the old guard sent them to `home` and
  // lost the query entirely).
  if (to.name === "sign-in" || to.name === "sign-up") {
    return isSafeRedirect(to.query.redirect)
      ? next(to.query.redirect)
      : next({ name: "home" })
  }

  return next()
})

// A 401 mid-session (signed out elsewhere, account deleted, struck from the
// roll) drops the cached user and sends them to sign-in with a way back.
setUnauthorizedHandler(() => {
  store.commit("setAuthUser", null)
  // Router 4 makes currentRoute a ref, hence `.value`. Reading it without that
  // yields the ref object itself, whose `.name` is undefined — which would look
  // exactly like the placeholder case below and silently disable this handler.
  const current = router.currentRoute.value
  if (publicRoutes.includes(current.name)) return
  // Before the first navigation resolves, currentRoute is Vue Router's
  // placeholder (START_LOCATION, name undefined). Pushing then cancels the
  // navigation in flight and the guard re-runs, so belt-and-braces against a
  // redirect loop: the guard already sends signed-out visitors to sign-in on
  // its own.
  if (!current.name) return
  router.push({
    name: "sign-in",
    query: { redirect: current.fullPath },
  })
})

export default router
