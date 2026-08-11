const { defineConfig } = require("@vue/cli-service")
const { VuetifyPlugin } = require("webpack-plugin-vuetify")

module.exports = defineConfig({
  // Vue CLI 5 / webpack is kept deliberately — see TODO3 Part K. Vite would
  // silently disable the immutable caching added by J4: `contentHashedAsset` in
  // server/main.go matches eight hex characters between dots (`app.457eeeac.js`),
  // which Vite's `index-DiwrgTda.js` can never satisfy. Moving to Vite means
  // moving that regexp first.
  //
  // `transpileDependencies: ["vuetify"]` is gone with Vuetify 2: v3 ships
  // browser-ready ESM and does not want re-transpiling.
  configureWebpack: {
    plugins: [
      // Replaces vuetify-loader. Tree-shakes Vuetify 3 components so the
      // bundle carries only what the templates actually use, which matters
      // here because main.js registers the whole library otherwise.
      new VuetifyPlugin({ autoImport: true }),
    ],
  },
})
