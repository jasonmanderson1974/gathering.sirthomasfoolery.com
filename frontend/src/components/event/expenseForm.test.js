import { describe, it, expect } from "vitest"
import {
  splitEvenlyPreview,
  parseAmount,
  formatCents,
  validateSplits,
  todayIso,
  expenseDateMin,
  expenseDateMax,
  EXPENSE_DATE_WINDOW_DAYS,
  isoToMillis,
  millisToIso,
  formatExpenseDate,
  fitWithin,
  receiptFileError,
  RECEIPT_MAX_EDGE,
} from "./expenseForm"

const shares = (splits) => splits.map((s) => s.amountCents)
const sum = (splits) => splits.reduce((total, s) => total + s.amountCents, 0)

// Ascending ids, so a test can say which participants the remainder goes to.
const people = (n) => Array.from({ length: n }, (_, i) => `id-${i}`)

describe("splitEvenlyPreview", () => {
  // The same table the Go side asserts in models/expense_split_test.go. If one
  // moves, the other has to move with it — the whole value of the preview is
  // that it shows what will actually be stored.
  it.each([
    ["exact division", 14250, 3, [4750, 4750, 4750]],
    ["one cent remainder", 1000, 3, [334, 333, 333]],
    ["n-1 cents remainder", 1001, 3, [334, 334, 333]],
    ["single participant takes everything", 9999, 1, [9999]],
    ["zero amount", 0, 3, [0, 0, 0]],
    ["one cent among four", 1, 4, [1, 0, 0, 0]],
    ["two people odd amount", 501, 2, [251, 250]],
  ])("%s", (_name, amount, count, want) => {
    const splits = splitEvenlyPreview(amount, people(count))

    expect(shares(splits)).toEqual(want)
    // The postcondition that actually matters: the ledger reconciles.
    expect(sum(splits)).toBe(amount)
  })

  it("gives the same answer whatever order the boxes were ticked in", () => {
    const forwards = splitEvenlyPreview(1002, people(5))
    const backwards = splitEvenlyPreview(1002, [...people(5)].reverse())

    expect(backwards).toEqual(forwards)
  })

  it("collapses a duplicate id into one share", () => {
    const splits = splitEvenlyPreview(1000, ["a", "b", "a"])

    expect(splits).toHaveLength(2)
    expect(sum(splits)).toBe(1000)
  })

  it("returns nothing for nobody, or for a nonsense amount", () => {
    expect(splitEvenlyPreview(1000, [])).toEqual([])
    expect(splitEvenlyPreview(1000, undefined)).toEqual([])
    expect(splitEvenlyPreview(-500, people(2))).toEqual([])
    expect(splitEvenlyPreview(10.5, people(2))).toEqual([])
  })
})

describe("parseAmount", () => {
  it.each([
    ["142.50", 14250],
    ["142.5", 14250],
    ["142", 14200],
    ["0.05", 5],
    [".5", 50],
    ["0", 0],
    ["$142.50", 14250],
    ["1,234.56", 123456],
    ["  42  ", 4200],
  ])("parses %s", (input, want) => {
    expect(parseAmount(input)).toBe(want)
  })

  it.each([
    ["", "blank"],
    ["   ", "whitespace"],
    ["abc", "letters"],
    ["-5", "negative — an expense is not a refund"],
    ["1.005", "more than two decimals"],
    ["1.2.3", "two decimal points"],
    ["1e3", "exponent notation"],
    [".", "a bare decimal point"],
    [null, "nothing at all"],
  ])("refuses %s (%s)", (input) => {
    expect(parseAmount(input)).toBeNull()
  })

  it("never rounds a cent away", () => {
    // parseFloat("1.005") * 100 is 100.49999999999999, which rounds to the
    // wrong cent. Nothing here goes through a float.
    expect(parseAmount("1.01")).toBe(101)
    expect(parseAmount("0.07")).toBe(7)
    expect(parseAmount("19.99")).toBe(1999)
  })
})

describe("formatCents", () => {
  it.each([
    [0, "$0.00"],
    [5, "$0.05"],
    [100, "$1.00"],
    [14250, "$142.50"],
    [123456, "$1,234.56"],
  ])("formats %d", (cents, want) => {
    expect(formatCents(cents)).toBe(want)
  })

  it("treats a missing amount as zero rather than NaN", () => {
    expect(formatCents(undefined)).toBe("$0.00")
    expect(formatCents(null)).toBe("$0.00")
  })
})

describe("validateSplits", () => {
  const splits = (...amounts) => amounts.map((amountCents) => ({ amountCents }))

  it("reports an exact split as exact", () => {
    expect(validateSplits(10000, splits(6000, 4000))).toEqual({
      total: 10000,
      delta: 0,
      exact: true,
    })
  })

  it("signs the delta so the form can say over or under", () => {
    expect(validateSplits(10000, splits(6000, 3000)).delta).toBe(-1000)
    expect(validateSplits(10000, splits(6000, 5000)).delta).toBe(1000)
  })

  it("is off by the whole amount when nothing has been entered", () => {
    expect(validateSplits(10000, []).delta).toBe(-10000)
    expect(validateSplits(10000, undefined).exact).toBe(false)
  })

  it("catches a single stray cent", () => {
    expect(validateSplits(1000, splits(334, 333, 332)).exact).toBe(false)
    expect(validateSplits(1000, splits(334, 333, 333)).exact).toBe(true)
  })
})

describe("dates", () => {
  it("defaults to today in the reader's own calendar", () => {
    // Late evening on the 2nd, in a timezone behind UTC, is still the 2nd.
    expect(todayIso(new Date(2026, 7, 2, 23, 30))).toBe("2026-08-02")
    expect(todayIso(new Date(2026, 0, 9, 0, 5))).toBe("2026-01-09")
  })

  it("round-trips a picked day unchanged", () => {
    for (const iso of ["2026-08-02", "2026-01-01", "2026-12-31"]) {
      expect(millisToIso(isoToMillis(iso))).toBe(iso)
    }
  })

  it("stores a day at UTC noon, so no timezone can shift it", () => {
    const millis = isoToMillis("2026-08-02")
    const at = new Date(millis)

    expect(at.getUTCHours()).toBe(12)
    // Twelve hours of slack either way covers every real offset (-12 to +14),
    // which is what keeps a Friday dinner from being filed under Thursday.
    expect(at.getUTCDate()).toBe(2)
  })

  it("renders a stored day the same way for every reader", () => {
    expect(formatExpenseDate(isoToMillis("2026-08-02"))).toBe("2 Aug 2026")
    expect(formatExpenseDate(null)).toBe("")
  })

  it("falls back to today for a missing stored date", () => {
    expect(millisToIso(null)).toBe(todayIso())
  })
})

describe("fitWithin", () => {
  it.each([
    ["already small enough", 800, 600, 800, 600],
    ["exactly at the cap", 2000, 1000, 2000, 1000],
    ["tall portrait receipt", 3024, 4032, 1500, 2000],
    ["wide landscape", 4000, 1000, 2000, 500],
    ["square", 4000, 4000, 2000, 2000],
    ["one pixel tall", 6000, 1, 2000, 1],
  ])("%s", (_name, w, h, wantW, wantH) => {
    expect(fitWithin(w, h)).toEqual({ width: wantW, height: wantH })
  })

  it("never produces a zero edge, which would break the canvas", () => {
    const { width, height } = fitWithin(9000, 2, RECEIPT_MAX_EDGE)

    expect(width).toBeGreaterThan(0)
    expect(height).toBeGreaterThan(0)
  })
})

describe("receiptFileError", () => {
  const file = (type, size) => ({ type, size })

  it("accepts an ordinary photo", () => {
    expect(receiptFileError(file("image/jpeg", 3_000_000))).toBeNull()
    expect(receiptFileError(file("image/png", 500_000))).toBeNull()
  })

  it("refuses a document dressed up as a receipt", () => {
    expect(receiptFileError(file("application/pdf", 1000))).toMatch(/photos/)
  })

  it("refuses something absurd before it is read into memory", () => {
    expect(receiptFileError(file("image/jpeg", 40 * 1024 * 1024))).toMatch(/too large/)
  })

  it("refuses nothing at all", () => {
    expect(receiptFileError(null)).toMatch(/No file/)
  })
})

// J10: the server rejects an expense dated outside ±expenseDateWindow with
// `invalid-date`. These bounds are what stop a person ever meeting that error —
// an unbounded picker let someone navigate to a year the API refuses, and the
// resulting save failure said nothing about the date being the cause.
describe("expense date bounds", () => {
  const now = new Date("2026-08-10T12:00:00Z")

  it("reaches a year in each direction from today", () => {
    expect(expenseDateMin(now)).toBe("2025-08-10")
    expect(expenseDateMax(now)).toBe("2027-08-10")
  })

  it("brackets today", () => {
    expect(expenseDateMin(now) < todayIso(now)).toBe(true)
    expect(expenseDateMax(now) > todayIso(now)).toBe(true)
  })

  it("stays in step with the window the server enforces", () => {
    // If this constant moves, expenseDateWindow in server/routes/expenses.go
    // has to move with it, or the picker offers dates the API rejects.
    expect(EXPENSE_DATE_WINDOW_DAYS).toBe(365)
  })

  it("produces picker-shaped strings, not timestamps", () => {
    expect(expenseDateMin(now)).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    expect(expenseDateMax(now)).toMatch(/^\d{4}-\d{2}-\d{2}$/)
  })
})
