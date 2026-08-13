/*
 * The one way this repo mounts a component in a test (TODO3 M6).
 *
 * Everything a view of this app needs to exist at all — Vuetify, a Vuex store,
 * a router — assembled once here, because the alternative is three test files
 * each inventing a slightly different app and none of them resembling `main.js`.
 */
import { mount } from "@vue/test-utils"
import { createVuetify } from "vuetify"
import * as components from "vuetify/components"
import * as directives from "vuetify/directives"
import { createStore } from "vuex"
import { createRouter, createMemoryHistory } from "vue-router"
import { roleGetters } from "@/store/role_getters"
import {
  peopleActions,
  peopleGetters,
  peopleMutations,
  peopleState,
} from "@/store/people"

/**
 * A Vuetify instance for tests.
 *
 * Built here rather than imported from `@/plugins/vuetify` on purpose: that
 * module selects the `mdi` FONT icon set, whose glyphs are bytes a test
 * environment has no way to load, and pulls in the theme from
 * `tailwind.config.js`. Neither is observable without real CSS, which is the
 * half of the browser check this tier explicitly does not replace. Components
 * and directives ARE registered globally, exactly as `main.js` does it — a
 * missing one is the difference between "the tab row rendered" and an empty div.
 */
function vuetify() {
  return createVuetify({ components, directives })
}

/**
 * A store with the real getters and mutations that matter to a view, over
 * whatever state the test hands it.
 *
 * `roleGetters` is imported rather than reimplemented: `canInvite` /
 * `canManageUsers` gate real branches of the templates under test, and a
 * hand-rolled stub of them would happily disagree with the app.
 *
 * The people slice (N3) is spread in for the same reason, and it matters more
 * there: which endpoint a hover card reaches for is decided by `canInvite`, and
 * sending a guest to the member-only roll is a 403, not a degraded card. A stub
 * would be free to get that right while the app got it wrong.
 */
export function testStore(state = {}) {
  return createStore({
    state: {
      error: "",
      info: "",
      authUser: null,
      offline: false,
      events: [],
      folders: [],
      daysOnlyEnabled: true,
      overlayAvailabilitiesEnabled: true,
      newDialogOptions: { show: false, contactsPayload: {}, folderId: null },
      ...peopleState(),
      ...state,
    },
    getters: { ...roleGetters, ...peopleGetters },
    mutations: {
      ...peopleMutations,
      setError: (s, v) => (s.error = v),
      setInfo: (s, v) => (s.info = v),
      setAuthUser: (s, v) => (s.authUser = v),
      setOffline: (s, v) => (s.offline = v),
      setEvents: (s, v) => (s.events = v),
      setFolders: (s, v) => (s.folders = v),
      setNewDialogOptions: (s, v) => (s.newDialogOptions = v),
    },
    actions: {
      ...peopleActions,
      showError: ({ commit }, e) => commit("setError", e),
      showInfo: ({ commit }, i) => commit("setInfo", i),
      getEvents: () => Promise.resolve(),
      getFolders: () => Promise.resolve(),
    },
  })
}

/**
 * A memory-history router carrying the app's real route NAMES.
 *
 * Names, not components: `$router.replace({ name: "home" })` throws on an
 * unknown name, and a view that navigates on an error path would fail the
 * console guard for the wrong reason. The components are placeholders because
 * no test here asserts on what a second route renders.
 */
export function testRouter() {
  const blank = { template: "<div />" }
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: "/", name: "landing", component: blank },
      { path: "/home", name: "home", component: blank },
      { path: "/settings", name: "settings", component: blank },
      { path: "/members", name: "admin", component: blank },
      { path: "/fellowship", name: "fellowship", component: blank },
      { path: "/chronicle", name: "chronicle", component: blank },
      { path: "/e/:eventId", name: "event", component: blank, props: true },
      { path: "/sign-in", name: "sign-in", component: blank },
      { path: "/:pathMatch(.*)*", name: "404", component: blank },
    ],
  })
}

/**
 * Mounts `component` into a real, attached document with the app's plugins.
 *
 * `attachTo` a live element is not optional: Vuetify teleports every overlay
 * (v-dialog, v-menu) to `document.body`, so a detached mount renders the
 * activator and nothing else — a dialog test would assert on markup that is
 * sitting in a document fragment nobody can query.
 *
 * @param {object} component the SFC under test
 * @param {object} [options] `props`, `state` (for the store), `stubs`, `route`
 * @returns {Promise<import("@vue/test-utils").VueWrapper>}
 */
export async function mountApp(component, options = {}) {
  const { props, state, stubs = {}, route = "/", ...rest } = options

  const router = testRouter()
  // The push is what makes `isReady()` resolve. A memory history has no
  // location to read on creation and performs NO initial navigation of its own,
  // so `await router.isReady()` alone never settles and every test in the file
  // dies on the 5s timeout with nothing to say about the component.
  router.push(route)
  await router.isReady()

  const host = document.createElement("div")
  document.body.appendChild(host)

  return mount(component, {
    props,
    attachTo: host,
    global: {
      plugins: [vuetify(), testStore(state), router],
      stubs,
    },
    ...rest,
  })
}

/**
 * The overlay content Vuetify teleported to `document.body`, if any is open.
 *
 * Overlays live OUTSIDE the wrapper's element, so `wrapper.find('[role=dialog]')`
 * finds nothing no matter how open the dialog is. Queried off the document for
 * the same reason `check-routes.js` does, and on the ARIA role rather than a
 * `.v-dialog--active` class, which Vuetify 3 does not emit — that class is what
 * made the browser check fail on the framework upgrade while the dialog itself
 * worked fine.
 */
export function openDialogs() {
  return [...document.querySelectorAll("[role=dialog]")].filter(
    (el) => el.getAttribute("aria-hidden") !== "true"
  )
}

/** Removes every host element and teleported overlay left behind by a mount. */
export function cleanupDom() {
  document.body.innerHTML = ""
}
