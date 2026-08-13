const { defineConfig } = require("@vue/cli-service")
const { VuetifyPlugin } = require("webpack-plugin-vuetify")
const { InjectManifest } = require("workbox-webpack-plugin")

const isProduction = process.env.NODE_ENV === "production"

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

      // The service worker, from our own source rather than a generated one —
      // see src/service-worker.js for why it stays precache-only.
      //
      // InjectManifest, not GenerateSW: a generated worker would decide its own
      // caching strategies, and the one rule that must hold here is that the
      // worker never touches /api.
      //
      // NOT @vue/cli-plugin-pwa, which would also inject tags into
      // public/index.html — a file that is templated twice (Vue CLI's <%= %> at
      // build, Go's {{ }} at serve) and whose head carries a standing "no
      // <link>s added here" rule. The manifest link is written by hand instead.
      ...(isProduction
        ? [
            new InjectManifest({
              swSrc: "./src/service-worker.js",
              // Must be exactly this name. It is the URL the worker registers
              // under, and therefore the only URL a stale worker ever refetches
              // — which is what makes deploy/kill-service-worker.js able to
              // work at all.
              swDest: "service-worker.js",
              // Every lazy route chunk HAS to be here — "opens offline" means
              // the event page's chunk is already on the device. What is
              // excluded is everything that is only ever wanted online, because
              // this list is a download a member pays for once on a phone.
              // Unexcluded it comes to 7.4MB; the exclusions below take it to
              // roughly 3.3MB without changing what works offline.
              exclude: [
                // A Go html/template, not a finished document. The worker
                // fetches the server's rendered `/` instead.
                /^index\.html$/,
                /\.map$/,
                /^manifest\.json$/,
                // Open Graph art, only ever read by other people's crawlers.
                /^img\/(ogImage|when2meetOgImage2)\.png$/,
                // Read by the OS at install time, never by the running app.
                /^img\/icons\//,
                /^favicon\.ico$/,
                // 3MB of the total, and none of it reachable: these are the
                // eot/ttf/woff fallbacks beside faces we also ship as woff2,
                // and every browser that has service workers has woff2. Note
                // the anchor — `.woff2` must NOT match, or the icon font goes
                // with them and all 69 mdi-* glyphs render as blank squares
                // offline with nothing logged (the L8 failure).
                /\.(eot|ttf|woff)$/,
                // Subsets for alphabets this club does not write in. The
                // browser only ever downloads the subset a page needs
                // (unicode-range), so precaching them is pure weight; if one is
                // ever needed offline it falls back to a system serif.
                /-(cyrillic|cyrillic-ext|greek|greek-ext|vietnamese)-/,
              ],
              // Raised from the 2MB default: chunk-vendors.css alone is ~700KB.
              maximumFileSizeToCacheInBytes: 6 * 1024 * 1024,
            }),
          ]
        : []),
    ],
  },
})
