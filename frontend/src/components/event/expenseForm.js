/**
 * The Add/Edit Expense form's arithmetic (F22), kept out of the .vue so it can
 * be tested — vitest runs in node with no jsdom here.
 *
 * splitEvenlyPreview mirrors models.SplitEvenly on the server, remainder rule
 * and all. That duplication is deliberate and worth being explicit about: the
 * server is authoritative and recomputes every split it stores, so this copy can
 * never make a wrong split real. What it buys is that the preview under the
 * amount field shows the numbers that will actually be stored — including which
 * two of three people get the extra cent — rather than a rounded guess that
 * changes the moment you save.
 *
 * If one of the two ever moves, the other has to move with it, and the tests on
 * both sides encode the same table.
 */

const CENTS_PER_UNIT = 100

/**
 * Divide amountCents evenly, so the shares sum to EXACTLY the total.
 *
 * $10 across three is not three shares of $3.33 — it is 334, 333, 333, with the
 * remainder handed one cent at a time to whoever sorts first by id. Sorting is
 * what makes it deterministic: the same input yields the same shares regardless
 * of the order the checkboxes were ticked in, which is what lets a re-save that
 * changed nothing be recognised as changing nothing.
 *
 * @param {number} amountCents
 * @param {string[]} participantIds user id hexes
 * @returns {Array<{userId: string, amountCents: number}>} sorted by id
 */
export const splitEvenlyPreview = (amountCents, participantIds) => {
  const ids = [...new Set((participantIds ?? []).filter(Boolean))].sort()
  if (ids.length === 0 || !Number.isInteger(amountCents) || amountCents < 0) {
    return []
  }

  const base = Math.floor(amountCents / ids.length)
  const remainder = amountCents % ids.length

  return ids.map((userId, i) => ({
    userId,
    amountCents: base + (i < remainder ? 1 : 0),
  }))
}

/**
 * Parse what someone typed into the amount field into integer cents.
 *
 * Returns null for anything unusable, which the form renders as "enter an
 * amount" rather than silently treating as zero.
 *
 * Parsed by hand rather than with parseFloat: `parseFloat("1.005") * 100` is
 * 100.49999999999999, and rounding that lands a cent away from what the person
 * typed. Money never becomes a float here, not even briefly.
 *
 * @param {string} input
 * @returns {number|null} cents
 */
export const parseAmount = (input) => {
  const text = String(input ?? "")
    .trim()
    // Currency symbols and thousands separators are what a person pasting from
    // a bank statement brings with them; neither changes the number.
    .replace(/[$,\s]/g, "")

  if (text === "") return null
  // No sign: an expense is a positive amount. A refund is not an expense.
  if (!/^\d*\.?\d*$/.test(text)) return null

  const [whole, fraction = ""] = text.split(".")
  if (whole === "" && fraction === "") return null
  // More than two decimals is a typo, not a fraction of a cent to round away.
  if (fraction.length > 2) return null

  const cents =
    Number(whole || "0") * CENTS_PER_UNIT + Number(fraction.padEnd(2, "0"))
  return Number.isSafeInteger(cents) ? cents : null
}

/**
 * Render integer cents as money.
 *
 * Intl.NumberFormat rather than string surgery, so a large total gets its
 * thousands separators. The currency is fixed: the Fellowship settles in one,
 * and a currency field nobody would ever change is a column of noise. This is
 * the single place to edit if that ever stops being true.
 */
const formatter = new Intl.NumberFormat("en-US", {
  style: "currency",
  currency: "USD",
})

export const formatCents = (cents) =>
  formatter.format((cents ?? 0) / CENTS_PER_UNIT)

/**
 * How far a by-amount split is from reconciling.
 *
 * Zero means it is exact, which is the only state the form allows a save in —
 * the server refuses anything else, and finding that out on submit rather than
 * while typing would be the worse of the two.
 *
 * @returns {{total: number, delta: number, exact: boolean}}
 *   delta is positive when the shares overshoot the expense.
 */
export const validateSplits = (amountCents, splits) => {
  const total = (splits ?? []).reduce(
    (sum, split) => sum + (split.amountCents ?? 0),
    0
  )
  const delta = total - (amountCents ?? 0)
  return { total, delta, exact: delta === 0 }
}

/**
 * The date field's default and its storage form.
 *
 * The picker works in "YYYY-MM-DD" and the API in unix milliseconds, and the
 * conversion runs through UTC noon rather than midnight — midnight local in a
 * negative offset is the previous day in UTC, which is how a Friday dinner ends
 * up filed under Thursday. Noon has twelve hours of slack in either direction.
 */
/**
 * How far from today the date field may reach, in either direction (J10).
 *
 * Mirrors `expenseDateWindow` in `server/routes/expenses.go`, which rejects
 * anything outside it with `invalid-date`. The two must agree: the server bound
 * is the guard (the ledger sorts by date, so an unbounded one is a sort-order
 * weapon), and these bounds are what keep a person from ever meeting it — an
 * unbounded picker would let someone navigate to a year the server refuses,
 * with nothing on screen saying why.
 */
export const EXPENSE_DATE_WINDOW_DAYS = 365

const shiftedIso = (days, now = new Date()) => {
  const at = new Date(now)
  at.setDate(at.getDate() + days)
  return todayIso(at)
}

export const expenseDateMin = (now = new Date()) =>
  shiftedIso(-EXPENSE_DATE_WINDOW_DAYS, now)

export const expenseDateMax = (now = new Date()) =>
  shiftedIso(EXPENSE_DATE_WINDOW_DAYS, now)

export const todayIso = (now = new Date()) => {
  const pad = (n) => String(n).padStart(2, "0")
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
}

/**
 * `YYYY-MM-DD` <-> `Date`, for Vuetify 3's date picker, which works in Date
 * objects where Vuetify 2 took ISO strings.
 *
 * Deliberately LOCAL-time (not the UTC-noon trick `isoToMillis` uses): the
 * picker compares against a locally-constructed "today" when applying `min` and
 * `max`, so a UTC date would land a day out either side of the window for
 * anyone west of Greenwich.
 */
export const isoToDate = (iso) => {
  const [year, month, day] = String(iso ?? "")
    .split("-")
    .map(Number)
  if (!year || !month || !day) return undefined
  return new Date(year, month - 1, day)
}

export const dateToIso = (date) => {
  const at = Array.isArray(date) ? date[0] : date
  if (!(at instanceof Date) || isNaN(at)) return todayIso()
  return todayIso(at)
}

export const isoToMillis = (iso) => {
  const [year, month, day] = String(iso ?? "")
    .split("-")
    .map(Number)
  if (!year || !month || !day) return Date.UTC(1970, 0, 1, 12)
  return Date.UTC(year, month - 1, day, 12)
}

export const millisToIso = (millis) => {
  if (millis == null) return todayIso()
  const at = new Date(millis)
  const pad = (n) => String(n).padStart(2, "0")
  return `${at.getUTCFullYear()}-${pad(at.getUTCMonth() + 1)}-${pad(
    at.getUTCDate()
  )}`
}

/**
 * A short, readable date for a ledger row: "2 Aug 2026".
 *
 * Read back in UTC, matching isoToMillis — the stored instant is a calendar day
 * that was picked, not a moment that happened, so re-interpreting it in the
 * reader's timezone would shift it for anyone west of Greenwich.
 */
export const formatExpenseDate = (millis) => {
  if (millis == null) return ""
  return new Date(millis).toLocaleDateString("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  })
}

/** Largest edge of a stored receipt — mirrors receiptMaxEdge on the server. */
export const RECEIPT_MAX_EDGE = 2000

/** Matches receiptJPEGQuality on the server. */
export const RECEIPT_JPEG_QUALITY = 0.82

/**
 * What a receipt file has to satisfy before it is worth downscaling, or null.
 *
 * The byte cap here is generous — 20MB — because the whole point of the
 * downscale below is that the original never leaves the browser. It exists only
 * to refuse something absurd before FileReader pulls it into memory.
 */
export const receiptFileError = (file) => {
  if (!file) return "No file chosen."
  if (!file.type?.startsWith("image/")) {
    return "Receipts must be photos — JPEG or PNG."
  }
  if (file.size > 20 * 1024 * 1024) {
    return "That photo is too large. Try one under 20MB."
  }
  return null
}

/**
 * Target dimensions for a receipt, capped on the long edge with the aspect
 * ratio preserved and never enlarged. Mirrors fitWithin on the server.
 *
 * No square crop, unlike an avatar: a receipt is a document, and the reason for
 * attaching one is that somebody can read it later. Cropping it square would
 * cut the total off the bottom.
 */
export const fitWithin = (width, height, maxEdge = RECEIPT_MAX_EDGE) => {
  const longest = Math.max(width, height)
  if (longest <= maxEdge || longest === 0) return { width, height }
  const scale = maxEdge / longest
  return {
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale)),
  }
}
