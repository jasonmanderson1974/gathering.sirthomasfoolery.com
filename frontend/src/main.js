import { createApp } from "vue"
import { createHead, VueHeadMixin } from "@unhead/vue/client"
import App from "./App.vue"
import router from "./router"
import store from "./store"
import vuetify from "./plugins/vuetify"
// The MDI webfont, self-hosted. It used to be a `<link>` to
// cdn.jsdelivr.net/npm/@mdi/font@latest — an unpinned, unverified third party
// that could change what renders here with no deploy on our side, and whose
// failure blanks all 69 `mdi-*` glyphs in the app with nothing logged (L8).
// Importing the package instead pins the version in package-lock.json and lets
// webpack emit the woff2 as a content-hashed, same-origin asset. Keep the
// version pinned exactly (no `^`): a floating icon font is the thing this fixed.
import "@mdi/font/css/materialdesignicons.css"
// The four text faces, self-hosted, for the same reasons as the icon font above
// plus one more (L9): as `<link>`s to fonts.googleapis.com they put a third
// party on the critical path of every page — first paint blocked on a host we
// don't control — and handed Google the IP and User-Agent of every member of an
// invite-only club, on every load, forever.
//
// These are the VARIABLE cuts, which is why the family names carry a
// `Variable` suffix (fontsource's naming, not ours) everywhere they are
// referenced: tailwind.config.js, index.css and App.vue. One file per family
// per subset covers the whole weight range, replacing the twelve static cuts
// the Google request enumerated. The italics are imported only for the two
// families that were actually asking Google for one — Cormorant Garamond and
// EB Garamond; DM Sans and Cinzel were requested upright-only and still are, so
// their obliques stay browser-synthesised exactly as before.
//
// Pinned exactly (no `^`), like @mdi/font: a floating text face can restyle the
// whole app with no deploy on our side.
import "@fontsource-variable/dm-sans"
import "@fontsource-variable/cinzel"
import "@fontsource-variable/cormorant-garamond"
import "@fontsource-variable/cormorant-garamond/wght-italic.css"
import "@fontsource-variable/eb-garamond"
import "@fontsource-variable/eb-garamond/wght-italic.css"
import "./index.css"

const app = createApp(App)

app.use(router)
app.use(store)
app.use(vuetify)

// Replaces vue-meta, which has no Vue 3 release worth taking (vue-meta 3 never
// left alpha). `VueHeadMixin` is what keeps the Options API working: components
// declare a `head` option exactly where they used to declare `metaInfo`.
//
// This only manages what a component actually declares, which matters here —
// the Go server injects per-route OG tags into index.html for /e/:eventId
// (`noRouteHandler` in main.go), and Event.vue declares no head of its own, so
// nothing on the client overwrites them.
app.use(createHead())
app.mixin(VueHeadMixin)

app.mount("#app")
