import { describe, it, expect } from "vitest"
import {
  netBalances,
  namesFromExpenses,
  simplifyDebts,
  settleUpSummary,
} from "./settleUp"

// Ids are readable strings rather than real ObjectID hexes. Nothing here parses
// them; the only thing that matters is that they sort deterministically, which
// is exactly the property the tie-breaking relies on.
const A = "aaa"
const B = "bbb"
const C = "ccc"
const D = "ddd"

/**
 * An expense paid by one person and split evenly between the named people.
 * Amounts that don't divide evenly are rejected here rather than rounded — the
 * server never produces such a row, so a test that built one would be testing a
 * state that cannot exist.
 */
const evenExpense = (paidBy, amountCents, people) => {
  expect(amountCents % people.length).toBe(0)
  return {
    paidBy,
    paidByName: paidBy.toUpperCase(),
    amountCents,
    splits: people.map((userId) => ({
      userId,
      name: userId.toUpperCase(),
      amountCents: amountCents / people.length,
    })),
  }
}

/** Total cents moved by a settlement, and the count of payments. */
const totalOf = (payments) =>
  payments.reduce((sum, p) => sum + p.amountCents, 0)

/**
 * Apply a settlement to a set of balances. Everyone must end at zero — this is
 * the property that actually matters, and it is stronger than checking any
 * particular list of payments.
 */
const applyPayments = (balances, payments) => {
  const after = new Map(balances)
  for (const { fromId, toId, amountCents } of payments) {
    after.set(fromId, (after.get(fromId) ?? 0) + amountCents)
    after.set(toId, (after.get(toId) ?? 0) - amountCents)
  }
  return after
}

const settlesCompletely = (balances) => {
  const after = applyPayments(balances, simplifyDebts(balances))
  return [...after.values()].every((cents) => cents === 0)
}

describe("netBalances", () => {
  it("credits the payer and debits each share", () => {
    const balances = netBalances([evenExpense(A, 9000, [A, B, C])])

    // A paid 9000 and owes 3000 of it, so is up 6000.
    expect(balances.get(A)).toBe(6000)
    expect(balances.get(B)).toBe(-3000)
    expect(balances.get(C)).toBe(-3000)
  })

  it("sums to zero across any number of expenses", () => {
    const balances = netBalances([
      evenExpense(A, 9000, [A, B, C]),
      evenExpense(B, 4000, [A, B]),
      evenExpense(C, 1500, [A, B, C]),
    ])

    const total = [...balances.values()].reduce((sum, cents) => sum + cents, 0)
    expect(total).toBe(0)
  })

  it("drops anyone who nets out exactly", () => {
    // B pays for a dinner they alone ate: they are owed nothing and owe nothing.
    const balances = netBalances([evenExpense(B, 2000, [B])])
    expect(balances.has(B)).toBe(false)
    expect(balances.size).toBe(0)
  })

  it("handles a payer who is not in the split", () => {
    // A treats B and C. A is owed the whole amount.
    const balances = netBalances([evenExpense(A, 6000, [B, C])])
    expect(balances.get(A)).toBe(6000)
    expect(balances.get(B)).toBe(-3000)
    expect(balances.get(C)).toBe(-3000)
  })

  it("carries uneven shares through untouched", () => {
    const balances = netBalances([
      {
        paidBy: A,
        amountCents: 1000,
        splits: [
          { userId: A, amountCents: 334 },
          { userId: B, amountCents: 333 },
          { userId: C, amountCents: 333 },
        ],
      },
    ])

    expect(balances.get(A)).toBe(666)
    expect(balances.get(B)).toBe(-333)
    expect(balances.get(C)).toBe(-333)
    expect([...balances.values()].reduce((s, c) => s + c, 0)).toBe(0)
  })

  it("is empty for an empty or absent ledger", () => {
    expect(netBalances([]).size).toBe(0)
    expect(netBalances(undefined).size).toBe(0)
  })
})

describe("simplifyDebts", () => {
  it("settles two people in one payment", () => {
    const balances = netBalances([evenExpense(A, 5000, [A, B])])
    const payments = simplifyDebts(balances)

    expect(payments).toEqual([{ fromId: B, toId: A, amountCents: 2500 }])
  })

  it("collapses a circle of debts into fewer payments than expenses", () => {
    // A→B→C→A: each pays for the other in turn. Settled naively that is three
    // payments; it should be fewer, and it should still leave everyone square.
    const balances = netBalances([
      evenExpense(A, 3000, [A, B]),
      evenExpense(B, 3000, [B, C]),
      evenExpense(C, 3000, [C, A]),
    ])

    // Everyone paid 3000 and owes 3000: the circle cancels entirely.
    expect(balances.size).toBe(0)
    expect(simplifyDebts(balances)).toEqual([])
  })

  it("never needs more than one payment fewer than there are people", () => {
    // Four people, one payer: three debtors, so at most three payments.
    const balances = netBalances([evenExpense(A, 12000, [A, B, C, D])])
    const payments = simplifyDebts(balances)

    expect(payments.length).toBeLessThanOrEqual(balances.size - 1)
    expect(settlesCompletely(balances)).toBe(true)
  })

  it("beats settling each expense separately", () => {
    // Five expenses between three people. Paying each one back on its own would
    // be five or more transfers; the whole point of the feature is that it isn't.
    const balances = netBalances([
      evenExpense(A, 6000, [A, B, C]),
      evenExpense(A, 3000, [A, B, C]),
      evenExpense(B, 9000, [A, B, C]),
      evenExpense(C, 1500, [A, B, C]),
      evenExpense(C, 4500, [A, B, C]),
    ])

    const payments = simplifyDebts(balances)
    expect(payments.length).toBeLessThanOrEqual(2)
    expect(settlesCompletely(balances)).toBe(true)
  })

  it("clears every balance exactly, including uneven ones", () => {
    const balances = netBalances([
      {
        paidBy: A,
        amountCents: 1000,
        splits: [
          { userId: A, amountCents: 334 },
          { userId: B, amountCents: 333 },
          { userId: C, amountCents: 333 },
        ],
      },
      {
        paidBy: B,
        amountCents: 700,
        splits: [
          { userId: B, amountCents: 234 },
          { userId: C, amountCents: 233 },
          { userId: D, amountCents: 233 },
        ],
      },
    ])

    expect(settlesCompletely(balances)).toBe(true)
    // No stray 1c payment left over by the rounding.
    for (const payment of simplifyDebts(balances)) {
      expect(payment.amountCents).toBeGreaterThan(0)
    }
  })

  it("moves exactly what is owed, no more", () => {
    const balances = netBalances([
      evenExpense(A, 9000, [A, B, C]),
      evenExpense(B, 3000, [A, B, C]),
    ])

    const owed = [...balances.values()]
      .filter((cents) => cents > 0)
      .reduce((sum, cents) => sum + cents, 0)

    expect(totalOf(simplifyDebts(balances))).toBe(owed)
  })

  it("is stable when two people are owed the same amount", () => {
    // C and D are each owed 1000; B owes 2000. Which of the two is paid first
    // must not depend on Map insertion order, or the panel reshuffles on every
    // refresh and reads as though something changed.
    const forwards = new Map([
      [B, -2000],
      [C, 1000],
      [D, 1000],
    ])
    const backwards = new Map([
      [D, 1000],
      [C, 1000],
      [B, -2000],
    ])

    expect(simplifyDebts(forwards)).toEqual(simplifyDebts(backwards))
    expect(simplifyDebts(forwards)[0].toId).toBe(C)
  })

  it("returns nothing for an empty or already-square ledger", () => {
    expect(simplifyDebts(new Map())).toEqual([])
    expect(simplifyDebts(undefined)).toEqual([])
  })
})

describe("namesFromExpenses", () => {
  it("takes names from the snapshots on the rows", () => {
    const names = namesFromExpenses([evenExpense(A, 4000, [A, B])])

    expect(names.get(A)).toBe("AAA")
    expect(names.get(B)).toBe("BBB")
  })

  it("picks up someone who only ever appears as the creator", () => {
    const names = namesFromExpenses([
      { ...evenExpense(A, 4000, [A, B]), createdBy: C, createdByName: "Carol" },
    ])

    expect(names.get(C)).toBe("Carol")
  })
})

describe("settleUpSummary", () => {
  it("names both ends of every payment", () => {
    const { payments } = settleUpSummary([evenExpense(A, 5000, [A, B])])

    expect(payments).toEqual([
      { fromId: B, toId: A, amountCents: 2500, fromName: "BBB", toName: "AAA" },
    ])
  })

  it("totals the ledger and counts who is involved", () => {
    const summary = settleUpSummary([
      evenExpense(A, 9000, [A, B, C]),
      evenExpense(B, 3000, [A, B, C]),
    ])

    expect(summary.totalCents).toBe(12000)
    expect(summary.people).toBe(3)
  })

  it("reports an empty ledger as square", () => {
    const summary = settleUpSummary([])

    expect(summary.payments).toEqual([])
    expect(summary.totalCents).toBe(0)
    expect(summary.people).toBe(0)
  })

  it("falls back to a placeholder for an account with no name anywhere", () => {
    const { payments } = settleUpSummary([
      {
        paidBy: A,
        amountCents: 1000,
        splits: [{ userId: B, amountCents: 1000 }],
      },
    ])

    expect(payments[0].fromName).toBe("Someone")
    expect(payments[0].toName).toBe("Someone")
  })
})
