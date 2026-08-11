/**
 * Run a function on a throwaway Web Worker.
 *
 * Replaces the `vue-worker` plugin (`this.$worker.run`), which was a six-line
 * wrapper installing `simple-web-worker` onto `Vue.prototype` — a Vue 2-only
 * mechanism, and the last thing standing between us and dropping two
 * unmaintained dependencies.
 *
 * Deliberately NOT in the `utils/index.js` barrel: that barrel is `export *`
 * and imported by ~40 components, and this module is DOM-dependent. Import it
 * by path.
 *
 * The behaviour matched here, because the one caller depends on all of it:
 *
 * - **`fn` is stringified into the worker body, so it cannot close over
 *   anything.** No imports, no module scope, no `this` — every helper it needs
 *   must be defined inside it. This is a property of the technique, not of this
 *   implementation; `simple-web-worker` worked the same way.
 * - **Arguments and the return value cross by structured clone**, not JSON, so
 *   `Map` and `Set` survive in both directions. The caller passes Sets in and
 *   gets a `Map` of `Set`s back, so this is load-bearing — swapping in a
 *   JSON-based transport would silently turn them into `{}`.
 *
 * One difference from the old implementation, and it is a fix: the worker is
 * terminated on the error path too. `simple-web-worker` ended its worker with
 * `close()` inside the worker body, which only runs when the work *succeeds* —
 * a throwing job left the worker alive and its object URL unrevoked.
 *
 * @param {Function} fn   self-contained function to run off the main thread
 * @param {Array} args    arguments, spread into `fn`
 * @returns {Promise<any>} whatever `fn` returned
 */
export function runInWorker(fn, args = []) {
  return new Promise((resolve, reject) => {
    const source = `self.onmessage = (e) => {
      self.postMessage((${fn}).apply(null, e.data))
      close()
    }`
    const url = URL.createObjectURL(
      new Blob([source], { type: "application/javascript" })
    )
    const worker = new Worker(url)

    const done = (settle) => (value) => {
      URL.revokeObjectURL(url)
      worker.terminate()
      settle(value)
    }

    worker.onmessage = (event) => done(resolve)(event.data)
    worker.onerror = (event) =>
      done(reject)(
        new Error(
          `worker failed at ${event.filename}:${event.lineno}: ${event.message}`
        )
      )

    worker.postMessage(args)
  })
}
