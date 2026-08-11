import { createApp } from "vue"
import { createHead, VueHeadMixin } from "@unhead/vue/client"
import App from "./App.vue"
import router from "./router"
import store from "./store"
import vuetify from "./plugins/vuetify"
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
