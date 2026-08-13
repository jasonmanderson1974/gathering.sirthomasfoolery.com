/*
  Warms the cache for every gathering the member might open next.

  Caching only what has been visited sounds thriftier and is the wrong trade:
  the gathering you want on the train is the one you did NOT happen to open
  while you had signal. The club runs a handful of gatherings at a time, and one
  event payload carries its discussion, lists, polls and RSVPs with it, so
  covering all of them is a few small requests rather than a sync engine.

  Everything here is deliberately timid. It runs behind the reads the member is
  actually waiting on, one gathering at a time, and gives up quietly on any
  failure — a warm cache is a nicety, and it must never cost the page it was
  supposed to help.
*/

import { get } from "@/utils/fetch_utils"
import { isOffline } from "./status"

/** Gap between gatherings, so this never competes with a real interaction. */
const PAUSE_MS = 150

/**
 * How long to stay out of the way before starting at all.
 *
 * This fires from `getEvents`, which runs on EVERY full page load — not just
 * the dashboard — so without a wait it puts a burst of background requests
 * alongside the ones the page is actually blocked on. That is not theoretical:
 * it was measurably slowing the fellowship page's own directory read.
 */
const START_DELAY_MS = 2000

/** Don't re-warm on every visit to the dashboard. */
const COOLDOWN_MS = 5 * 60 * 1000

let running = false
let lastRunAt = 0

const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

/**
 * Resolves once the browser is idle, or after `ms` regardless.
 *
 * requestIdleCallback is the honest expression of "when nothing better is
 * happening"; the timeout is both its own backstop and the fallback for Safari,
 * which did not ship it until recently.
 */
const whenIdle = (ms) =>
  new Promise((resolve) => {
    if (typeof requestIdleCallback === "function") {
      requestIdleCallback(() => resolve(), { timeout: ms })
    } else {
      setTimeout(resolve, ms)
    }
  })

const quietly = (route) => get(route).catch(() => {})

/**
 * @param {Array} events the projected list from GET /user/events
 * @param {object} [options] `pauseMs` for tests
 */
export const prefetchGatherings = async (
  events,
  { pauseMs = PAUSE_MS, startDelayMs = START_DELAY_MS } = {}
) => {
  if (running || isOffline() || !Array.isArray(events)) return
  if (Date.now() - lastRunAt < COOLDOWN_MS) return

  running = true
  try {
    // Let the page finish what it is doing first.
    await whenIdle(startDelayMs)
    if (isOffline()) return

    // Archived gatherings are excluded: they are history, and the payloads are
    // the same size as a live one.
    const active = events.filter((event) => event && !event.isArchived)

    for (const event of active) {
      // Rechecked each time round rather than once at the top — losing signal
      // mid-warm is the ordinary case on a phone, and there is no point
      // grinding through a dozen failing requests to discover it.
      if (isOffline()) break

      const id = event.shortId ?? event._id
      if (!id) continue

      // The event payload first: it is the one that carries the discussion and
      // the lists, so a warm-up cut short still leaves the page readable.
      await quietly(`/events/${id}`)
      await quietly(`/events/${id}/expenses`)
      await pause(pauseMs)
    }
    lastRunAt = Date.now()
  } finally {
    running = false
  }
}

/** Test seam. */
export const __resetPrefetchForTests = () => {
  running = false
  lastRunAt = 0
}
