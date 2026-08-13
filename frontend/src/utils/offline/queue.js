/*
  The offline write queue (TODO3 O5).

  A durable FIFO of operations made while the server was unreachable, flushed in
  order when it comes back. Three things make it more than an array:

  1. ORDER IS LOAD-BEARING. A later op can target something an earlier one
     created — tick the item you just added — so the flush is strictly serial.
     It is also gentler on a server where every authed request costs a user
     lookup and an allowlist check.

  2. THINGS CREATED OFFLINE HAVE NO REAL ID YET. A create mints a temporary id
     locally and carries it as the `clientId` the server stores (O4), so a
     replay makes one row rather than two. Ops queued behind it refer to the
     temporary id, and the flush rewrites those to the real id as it learns it.
     Most of that never happens, because of (3).

  3. A QUEUE THAT ONLY GROWS IS A BUG. Editing something still queued rewrites
     the queued op in place; deleting it drops both. Typing a note ten times
     leaves one save, not ten.

  A POISON OP MUST NEVER WEDGE THE QUEUE. A 4xx means the server has considered
  the request and refused it — a list deleted meanwhile, permission changed —
  and no amount of retrying will change that, so it is dropped and reported.
  Only a transport failure or a 5xx is worth retrying.
*/

import { QUEUE_STORE, withNamedStore } from "./cache"
import { isOffline, noteNetworkFailure } from "./status"
import { ABSOLUTE_STATE_OPS, OPS } from "./ops"

/** Ops beyond this are dropped rather than kept forever. */
const MAX_QUEUE = 500
/** And ops older than this: a fortnight-old edit is not worth applying. */
const MAX_AGE_MS = 14 * 24 * 60 * 60 * 1000
/** Comment creates are spaced out on flush; see flush(). */
const MENTION_SPACING_MS = 1100

let seq = 0
let flushing = false
const listeners = new Set()

/**
 * Ids are `<time>-<counter>-<random>`: lexicographically ordered so IndexedDB's
 * own key order IS the queue order, with a counter so two ops enqueued in the
 * same millisecond keep their sequence.
 */
const newId = () =>
  `${String(Date.now()).padStart(14, "0")}-${String(seq++).padStart(6, "0")}-${Math.random()
    .toString(36)
    .slice(2, 8)}`

/*
  Temporary id -> the real id the server gave it, kept ACROSS flushes.

  Per-flush was not enough, and the gap is easy to walk into: a create flushes
  on Tuesday, the cached page still shows the item under its temporary id until
  something refetches it, and an edit made offline on Wednesday would address
  that dead id and 404. The map has to outlive the flush that learned it.

  localStorage rather than IndexedDB: it is a handful of opaque id pairs, it has
  to be readable synchronously while enqueueing, and it does not warrant a
  schema bump. Capped, oldest-first, because it must not grow forever.
*/
const ID_MAP_KEY = "offline-id-map"
const ID_MAP_MAX = 300

const readIdMap = () => {
  try {
    const raw = localStorage.getItem(ID_MAP_KEY)
    return raw ? new Map(JSON.parse(raw)) : new Map()
  } catch {
    return new Map()
  }
}

const rememberRealId = (tempId, realId) => {
  if (!tempId || !realId || tempId === realId) return
  try {
    const map = readIdMap()
    map.delete(tempId)
    map.set(tempId, realId)
    const entries = [...map.entries()].slice(-ID_MAP_MAX)
    localStorage.setItem(ID_MAP_KEY, JSON.stringify(entries))
  } catch {
    // No storage. The in-flight flush still resolves ids correctly; only a
    // LATER offline edit of this same item would miss, which is the rarer half.
  }
}

/** Rewrites a temporary id to its real one, if we have learned it. */
export const resolveTempId = (id) => (id ? (readIdMap().get(id) ?? id) : id)

const clearIdMap = () => {
  try {
    localStorage.removeItem(ID_MAP_KEY)
  } catch {
    /* nothing to clear */
  }
}

/** A temporary id for something created offline, doubling as its clientId. */
export const newTempId = () =>
  typeof crypto !== "undefined" && crypto.randomUUID
    ? crypto.randomUUID()
    : `tmp-${Date.now()}-${Math.random().toString(36).slice(2)}`

/*
  Two layers, as the cache has, and here it matters more.

  The cache degrading to memory-only costs a member the ability to read offline
  after their tab is evicted. The QUEUE degrading to nothing would silently
  discard writes they had already been shown as saved — so a browser without
  usable IndexedDB (Safari private browsing, a quota refusal) must still keep
  the queue for as long as the tab lives, and flush it on reconnect.

  Memory is the working copy; IndexedDB is what survives the tab.
*/
const memory = new Map()
let loaded = false

const ensureLoaded = async () => {
  if (loaded) return
  loaded = true
  const rows = await withNamedStore(
    QUEUE_STORE,
    "readonly",
    (s) => s.getAll(),
    []
  )
  for (const row of rows || []) {
    if (row?.id && !memory.has(row.id)) memory.set(row.id, row)
  }
}

const readAll = async () => {
  await ensureLoaded()
  return [...memory.values()].sort((a, b) => (a.id < b.id ? -1 : 1))
}

const putOp = async (op) => {
  await ensureLoaded()
  memory.set(op.id, op)
  await withNamedStore(QUEUE_STORE, "readwrite", (s) => s.put(op))
}

const deleteOp = async (id) => {
  await ensureLoaded()
  memory.delete(id)
  await withNamedStore(QUEUE_STORE, "readwrite", (s) => s.delete(id))
}

const notify = async () => {
  const pending = await pendingCount()
  for (const fn of listeners) {
    try {
      fn(pending)
    } catch {
      /* a subscriber's failure is not the queue's to propagate */
    }
  }
}

export const onQueueChange = (fn) => {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

export const pendingCount = async () => (await readAll()).length

/* ------------------------------------------------------------------ *
 * Enqueue, with collapse
 * ------------------------------------------------------------------ */

/**
 * Rewrites or removes queued ops this new one supersedes.
 *
 * Returns true when the new op has been absorbed entirely and should not be
 * appended. Two rules, and both matter for a queue built over a long train
 * journey rather than a moment:
 *
 *  - An op targeting something whose CREATE is still queued folds into that
 *    create. Edit the item you just added and the queued add carries the new
 *    text; delete it and both disappear, having never reached the server. This
 *    is also what makes id remapping rare rather than the common case.
 *  - An absolute-state op replaces an earlier one for the same target. Ten
 *    autosaves of a note are one save.
 */
const collapse = async (op) => {
  const queued = await readAll()

  const targetId = op.itemId ?? op.commentId ?? op.expenseId ?? null
  if (targetId) {
    const create = queued.find(
      (q) => OPS[q.kind]?.creates && q.tempId === targetId
    )
    if (create) {
      if (op.kind.endsWith(".delete")) {
        // Never happened. Drop the create and everything queued against it.
        for (const q of queued) {
          if (q.id === create.id || q.itemId === targetId) await deleteOp(q.id)
        }
        return true
      }
      if (op.kind.endsWith(".edit")) {
        await putOp({ ...create, text: op.text ?? create.text })
        return true
      }
      if (op.kind.endsWith(".check")) {
        await putOp({ ...create, checked: op.checked })
        return true
      }
    }
  }

  if (ABSOLUTE_STATE_OPS.has(op.kind)) {
    for (const q of queued) {
      if (
        q.kind === op.kind &&
        q.eventId === op.eventId &&
        (q.itemId ?? null) === (op.itemId ?? null) &&
        (q.listId ?? null) === (op.listId ?? null)
      ) {
        await deleteOp(q.id)
      }
    }
  }

  return false
}

/**
 * Records a write to be sent later, and applies it to the cache now so the
 * page reflects it. Returns what the caller should treat as the server's
 * answer — for a create, an object carrying the temporary id.
 */
export const enqueue = async (kind, fields) => {
  if (!OPS[kind]) throw new Error(`unknown offline op: ${kind}`)

  // Resolve any target id up front. The page may still be showing a temporary
  // id for something that has since been created for real, and addressing the
  // dead one would 404 on the next flush.
  const resolved = { ...fields }
  for (const key of ["itemId", "commentId", "expenseId", "listId", "parentId", "threadId"]) {
    if (resolved[key]) resolved[key] = resolveTempId(resolved[key])
  }

  const op = {
    id: newId(),
    kind,
    createdAt: Date.now(),
    attempts: 0,
    ...resolved,
  }
  if (OPS[kind].creates && !op.tempId) {
    op.tempId = newTempId()
    // The same value goes to the server as clientId, which is what makes a
    // replay idempotent (O4).
    op.clientId = op.tempId
  }

  const absorbed = await collapse(op)
  if (!absorbed) {
    const queued = await readAll()
    if (queued.length >= MAX_QUEUE) {
      // Drop the oldest rather than refuse the newest: the thing just typed is
      // the thing most likely to matter.
      await deleteOp(queued[0].id)
    }
    await putOp(op)
  }

  await OPS[kind].apply(op)
  await notify()
  return op.tempId ? { _id: op.tempId, pending: true } : { pending: true }
}

/* ------------------------------------------------------------------ *
 * Flush
 * ------------------------------------------------------------------ */

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

/**
 * Sends everything queued, oldest first, stopping at the first sign the network
 * is gone again.
 *
 * @returns {Promise<{sent: number, dropped: Array<{op: object, reason: string}>}>}
 */
export const flush = async ({ spacingMs = MENTION_SPACING_MS } = {}) => {
  if (flushing || isOffline()) return { sent: 0, dropped: [] }
  flushing = true

  const dropped = []
  let sent = 0
  // Reads through to the persisted map, so ids learned by an earlier flush
  // resolve too.
  const resolveId = (id) => resolveTempId(id)

  try {
    const queued = await readAll()
    let previousWasComment = false

    for (const op of queued) {
      if (Date.now() - op.createdAt > MAX_AGE_MS) {
        await deleteOp(op.id)
        dropped.push({ op, reason: "too old to send" })
        continue
      }

      const handler = OPS[op.kind]
      if (!handler) {
        await deleteOp(op.id)
        dropped.push({ op, reason: "unknown operation" })
        continue
      }

      // Mention emails are rate limited per author per hour
      // (routes/mention_emails.go). A batch of queued @mention comments can
      // exhaust that budget invisibly — the comments save, the people
      // mentioned are simply never told — so comment creates are spaced.
      if (previousWasComment && spacingMs > 0) await sleep(spacingMs)
      previousWasComment = op.kind === "comment.add"

      try {
        const result = await handler.send(op, resolveId)
        if (op.tempId && result?._id) rememberRealId(op.tempId, result._id)
        await deleteOp(op.id)
        sent++
      } catch (err) {
        if (err?.offline) {
          // The connection went again. Everything still queued stays queued,
          // in order, for the next attempt.
          noteNetworkFailure()
          break
        }
        if (err?.status >= 500) {
          // The server's problem, not this request's. Leave it for next time,
          // but count the attempt so a permanently failing op eventually goes.
          await putOp({ ...op, attempts: (op.attempts ?? 0) + 1 })
          if ((op.attempts ?? 0) + 1 >= 5) {
            await deleteOp(op.id)
            dropped.push({ op, reason: "the server kept failing" })
          }
          break
        }
        // A 4xx: considered and refused. Retrying cannot change that, and an op
        // that cannot succeed must not block the ones behind it.
        await deleteOp(op.id)
        dropped.push({ op, reason: describe(err) })
      }
    }
  } finally {
    flushing = false
    await notify()
  }

  return { sent, dropped }
}

const describe = (err) => {
  if (err?.status === 404) return "it was already deleted"
  if (err?.status === 403) return "you no longer have permission"
  if (err?.status === 400) return "the server refused it"
  return `it could not be saved (${err?.status ?? "unknown error"})`
}

/** Drops everything. Used on sign-out — a queue is one member's pending work. */
export const clearQueue = async () => {
  memory.clear()
  clearIdMap()
  await withNamedStore(QUEUE_STORE, "readwrite", (s) => s.clear())
  await notify()
}

/** Test seam. */
export const __resetQueueForTests = () => {
  seq = 0
  flushing = false
  loaded = false
  memory.clear()
  listeners.clear()
}
