import { defineConfig } from "vitest/config"
import vue from "@vitejs/plugin-vue"
import path from "path"
import { fileURLToPath } from "url"

const rootDirectory = path.dirname(fileURLToPath(import.meta.url))

/*
 * Two tiers, deliberately (TODO3 M6).
 *
 * `node` is the original suite: pure JS extracted OUT of components, no DOM,
 * 395 tests in ~1.5s. It cannot render anything, and that was a considered Vue
 * 2 decision — it stays exactly as it was.
 *
 * `dom` is the missing middle between it and `scripts/browser-check.sh`, which
 * is the only other thing in the repo that renders a component and costs 3m19s
 * to say so. Every browser-only bug this repo has paid for that was NOT a
 * layout bug — K3's dialog that threw on open, L1's validation guard that never
 * fired, K5's toggles that could not be turned off — is a mounted-component
 * fault findable in milliseconds. It does not replace the browser check: no
 * real CSS, no layout, no webfont, no 390px viewport.
 *
 * Split by FILENAME (`.test.js` vs `.spec.js`) rather than by directory, so the
 * two globs cannot overlap and a file's tier is readable from its name.
 */
export default defineConfig({
  resolve: {
    alias: {
      "@": path.resolve(rootDirectory, "./src"),
    },
    // Vue CLI's webpack config resolves `.vue` without the extension, and the
    // app has ~20 imports written that way (`from "./CalendarPermissionsCard"`).
    // Vite does not, so without this the FIRST such import in a mounted tree
    // fails the whole spec file at collection with "Does the file exist?" —
    // pointing at a file that does. Matching the app's resolution here is the
    // only way the two agree on what a module specifier means.
    extensions: [".mjs", ".js", ".json", ".vue"],
  },
  test: {
    projects: [
      {
        extends: true,
        test: {
          name: "node",
          environment: "node",
          include: ["src/**/*.test.js"],
        },
      },
      {
        extends: true,
        // Only this tier compiles `.vue`, and it does so with vite's plugin
        // while the APP is still built by Vue CLI / webpack. That is not a
        // half-migration: the plugin is the SFC compiler (`@vue/compiler-sfc`,
        // already a dependency) wired into the transform vitest happens to
        // use. It never runs over the shipped bundle, and it does not put the
        // repo one step closer to Vite — whose asset hashes would silently
        // switch off the immutable caching (TODO3 K, Phase 0).
        plugins: [vue()],
        test: {
          name: "dom",
          environment: "happy-dom",
          include: ["src/**/*.spec.js"],
          setupFiles: ["./src/test/setup.dom.js"],
          // Vuetify's component modules import their own `.css` next to them.
          // Left external they are handed to node's ESM loader, which has no
          // idea what a stylesheet is and fails the whole file with
          // `Unknown file extension ".css"`. Inlined, vite processes them — to
          // nothing, since this tier has no real CSS by design.
          server: { deps: { inline: ["vuetify"] } },
        },
      },
    ],
  },
})
