import Vue from "vue"
import VueRouter from "vue-router"
import Landing from "@/views/Landing"
import { get } from "@/utils"

Vue.use(VueRouter)

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
    path: "/s/:signUpId",
    name: "signUp",
    component: () => import("@/views/SignUp.vue"),
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
    path: "*",
    name: "404",
    component: () => import("@/views/PageNotFound.vue"),
  },
]

const router = new VueRouter({
  mode: "history",
  base: process.env.BASE_URL,
  routes,
})

router.beforeEach(async (to, from, next) => {
  const authRoutes = ["home", "settings", "admin", "fellowship"]
  const noAuthRoutes = ["sign-in", "sign-up"]
  try {
    await get("/auth/status")

    if (noAuthRoutes.includes(to.name)) {
      next({ name: "home" })
    } else {
      next()
    }
  } catch (err) {
    if (authRoutes.includes(to.name)) {
      next({ name: "landing" })
    } else {
      next()
    }
  }
})

export default router
