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
      const id = event.shortId ?? event._id
      if (!id) continue

      // Ordered by what a page needs first, and checked between EVERY read
      // rather than once per gathering: losing signal mid-warm is the ordinary
      // case on a phone, and there is no point grinding through the rest of a
      // gathering's reads to rediscover it.
      //
      // The two private tabs are here because they are NOT in the event
      // payload — they live in their own collections precisely so nothing can
      // leak them — so unlike the shared lists they are simply absent offline
      // unless fetched in their own right. That gap is what left My Lists empty
      // for a member who installed to the home screen, opened a gathering and
      // switched to airplane mode without having opened that tab first.
      const reads = ["", "/expenses", "/my-lists", "/my-notes"]
      for (const suffix of reads) {
        if (isOffline()) break
        await quietly(`/events/${id}${suffix}`)
      }
      if (isOffline()) break
      await pause(pauseMs)
    }
    lastRunAt = Date.now()
  } finally {
    running = false
  }
}

/**
 * Warms the reads that only matter on the gathering actually being looked at.
 *
 * Kept out of the sweep above deliberately: multiplied across every active
 * gathering these would double the warm-up for lists almost nobody will open,
 * whereas on the page in front of you they are the difference between being
 * able to add an expense offline and not. Called from Event.vue on arrival.
 *
 * `participants` is the one that earns its place: the expense form needs the
 * people to split between, so without it Settle Up is readable offline but not
 * writable — and Settle Up is the tab this whole feature was asked for.
 */
export const prefetchOpenGathering = async (eventId) => {
  if (!eventId || isOffline()) return
  await quietly(`/events/${eventId}/expenses/participants`)
  await quietly(`/events/${eventId}/lists/assignees`)
  await quietly(`/events/${eventId}/mentionables`)
}

/** Test seam. */
export const __resetPrefetchForTests = () => {
  running = false
  lastRunAt = 0
}
