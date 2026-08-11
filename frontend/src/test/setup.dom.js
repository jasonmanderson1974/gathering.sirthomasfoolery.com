/*
 * Setup for the `dom` vitest project (TODO3 M6): the browser APIs happy-dom
 * does not implement but Vuetify calls unconditionally, and a console guard.
 *
 * THE CONSOLE GUARD IS THE POINT OF THIS FILE. A mounted component that throws
 * from a lifecycle hook does not fail a test on its own — Vue catches it and
 * reports it through `console.error`, and the mount returns a wrapper that
 * looks fine. That is exactly how K3's New Gathering dialog threw a TypeError
 * on every open for the whole Vue 3 migration with everything green. Vue
 * reports a bad prop, a missing injection or a deprecated API through
 * `console.warn` the same way, and those are what a framework bump speaks
 * through (K5, L1, L3, L7 were all found by reading warnings, by hand).
 *
 * So every test in this project fails on either, whether or not it asserted on
 * it. `expectConsole(/…/)` opts one line out, the way `expectConsoleErrors`
 * does for a route in `check-routes.js`.
 *
 * These tests run against the DEV build, where the warnings exist at all — the
 * production bundle compiles them out, which is the whole of TODO3 M2.
 */
import { afterEach, beforeEach, expect, vi } from "vitest"
import { resetApi } from "./api"

/** Console lines recorded since the current test began. */
let recorded = []
/** Regexes for lines the current test has declared it expects. */
let expected = []

/**
 * Mutes console lines matching `re` for the rest of the current test.
 *
 * Use it for output a component correctly produces — a handled fetch failure
 * it reports, say — never to quiet a warning that is telling you something.
 */
export function expectConsole(re) {
  expected.push(re)
}

/** Everything the console recorded this test, muted lines included. */
export function consoleLines() {
  return recorded.map((r) => r.text)
}

const FRAMEWORK = /\[Vue warn\]|\[Vuetify\]|\[vue-router\]|\[vuex\]/i

// Same reading as browser-check-lib's `pageErrors`: every argument, falling
// back from the value to its description, because `console.error(someError)` —
// which is how Vue reports a throw from a hook — carries nothing useful in the
// first argument's `value`.
const textOf = (args) =>
  args
    .map((a) => {
      if (a instanceof Error) return a.stack || a.message
      if (typeof a === "string") return a
      try {
        return String(a)
      } catch {
        return ""
      }
    })
    .join(" ")
    .trim()

beforeEach(() => {
  recorded = []
  expected = []
  resetApi()
  for (const level of ["error", "warn"]) {
    vi.spyOn(console, level).mockImplementation((...args) => {
      recorded.push({ level, text: textOf(args) })
    })
  }
})

afterEach(() => {
  const offending = recorded.filter(
    (r) =>
      (r.level === "error" || FRAMEWORK.test(r.text)) &&
      !expected.some((re) => re.test(r.text))
  )
  vi.restoreAllMocks()
  // Printed as well as asserted: a stack trace inside a Vue warning is long,
  // and expect's diff truncates it exactly where the component name is.
  if (offending.length > 0) {
    for (const r of offending)
      console.info(`unexpected console.${r.level}:`, r.text)
  }
  expect(
    offending.map((r) => `console.${r.level}: ${r.text}`),
    "component logged errors or framework warnings"
  ).toEqual([])
})

/* ---------- browser APIs happy-dom does not have ---------- */

// Vuetify's display composable constructs one for every v-dialog, v-menu and
// v-overlay. Without it the mount throws before a single assertion runs.
if (typeof globalThis.ResizeObserver === "undefined") {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
}

if (typeof globalThis.IntersectionObserver === "undefined") {
  globalThis.IntersectionObserver = class {
    constructor(callback) {
      this.callback = callback
    }
    observe() {}
    unobserve() {}
    disconnect() {}
    takeRecords() {
      return []
    }
  }
}

// Read unguarded by Vuetify's overlay location strategies, so without it every
// dialog and menu mount throws `visualViewport is not defined` — from a watcher,
// which means it surfaces as an unhandled rejection rather than at the mount.
if (typeof globalThis.visualViewport === "undefined") {
  globalThis.visualViewport = {
    width: 1280,
    height: 900,
    offsetLeft: 0,
    offsetTop: 0,
    scale: 1,
    addEventListener() {},
    removeEventListener() {},
  }
}

if (typeof globalThis.matchMedia === "undefined") {
  globalThis.matchMedia = (query) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener() {},
    removeListener() {},
    addEventListener() {},
    removeEventListener() {},
    dispatchEvent: () => false,
  })
}

// jsdom and happy-dom both leave this unimplemented and Vuetify's transitions
// call it. Not stubbing it turns every dialog open into an unhandled throw.
if (typeof Element !== "undefined" && !Element.prototype.animate) {
  Element.prototype.animate = () => ({
    finished: Promise.resolve(),
    cancel() {},
    finish() {},
    addEventListener() {},
    removeEventListener() {},
  })
}
