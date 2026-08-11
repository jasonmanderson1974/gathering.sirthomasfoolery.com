import { describe, it, expect } from "vitest"
import {
  netBalances,
  namesFromExpenses,
  simplifyDebts,
  personTotals,
  personBreakdown,
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

describe("personTotals", () => {
  it("separates what someone paid from what they owe for", () => {
    // A paid for everything; B was at both. Paying does not make it yours.
    const totals = personTotals([
      evenExpense(A, 9000, [A, B]),
      evenExpense(A, 1000, [A, B]),
    ])

    const byId = Object.fromEntries(totals.map((t) => [t.userId, t]))
    expect(byId[A].paidCents).toBe(10000)
    expect(byId[A].shareCents).toBe(5000)
    expect(byId[A].netCents).toBe(5000)
    expect(byId[B].paidCents).toBe(0)
    expect(byId[B].shareCents).toBe(5000)
    expect(byId[B].netCents).toBe(-5000)
  })

  it("reproduces the two-person case from the brief", () => {
    // Phil paid $266.00, Jason $343.50, split evenly: $609.50 in all, and Phil
    // owes Jason $38.75.
    const phil = "phil"
    const jason = "jason"
    const totals = personTotals([
      {
        paidBy: phil,
        paidByName: "Phil",
        amountCents: 26600,
        splits: [
          { userId: phil, name: "Phil", amountCents: 13300 },
          { userId: jason, name: "Jason", amountCents: 13300 },
        ],
      },
      {
        paidBy: jason,
        paidByName: "Jason",
        amountCents: 34350,
        splits: [
          { userId: phil, name: "Phil", amountCents: 17175 },
          { userId: jason, name: "Jason", amountCents: 17175 },
        ],
      },
    ])

    const byId = Object.fromEntries(totals.map((t) => [t.userId, t]))
    expect(byId[phil].paidCents).toBe(26600)
    expect(byId[jason].paidCents).toBe(34350)
    // Each is responsible for half of $609.50.
    expect(byId[phil].shareCents).toBe(30475)
    expect(byId[jason].shareCents).toBe(30475)
    expect(byId[phil].netCents).toBe(-3875)
    expect(byId[jason].netCents).toBe(3875)
  })

  it("has both columns summing to the ledger total", () => {
    const rows = [
      evenExpense(A, 9000, [A, B, C]),
      evenExpense(B, 3000, [A, B, C]),
      evenExpense(C, 1500, [A, B, C]),
    ]
    const totals = personTotals(rows)
    const ledger = rows.reduce((sum, e) => sum + e.amountCents, 0)

    expect(totals.reduce((s, t) => s + t.paidCents, 0)).toBe(ledger)
    expect(totals.reduce((s, t) => s + t.shareCents, 0)).toBe(ledger)
  })

  it("agrees with netBalances, which the settlement is built from", () => {
    const rows = [evenExpense(A, 9000, [A, B, C]), evenExpense(B, 4500, [B, C])]
    const balances = netBalances(rows)

    for (const person of personTotals(rows)) {
      // netBalances drops anyone who nets to zero; personTotals keeps them.
      expect(person.netCents).toBe(balances.get(person.userId) ?? 0)
    }
  })

  it("keeps someone who paid exactly their own share, at zero", () => {
    // netBalances drops them — they are not part of the settlement — but the
    // table must still show what they put in.
    const totals = personTotals([evenExpense(B, 2000, [B])])

    expect(totals).toHaveLength(1)
    expect(totals[0].paidCents).toBe(2000)
    expect(totals[0].shareCents).toBe(2000)
    expect(totals[0].netCents).toBe(0)
  })

  it("includes a payer who is not in the split", () => {
    const totals = personTotals([evenExpense(A, 6000, [B, C])])
    const byId = Object.fromEntries(totals.map((t) => [t.userId, t]))

    expect(byId[A].paidCents).toBe(6000)
    expect(byId[A].shareCents).toBe(0)
    expect(totals).toHaveLength(3)
  })

  it("sorts by name so rows do not jump when an expense is added", () => {
    const named = (id, label) => ({
      paidBy: id,
      paidByName: label,
      amountCents: 100,
      splits: [{ userId: id, name: label, amountCents: 100 }],
    })

    const totals = personTotals([
      named(C, "Zoe"),
      named(A, "adam"),
      named(B, "Mel"),
    ])
    expect(totals.map((t) => t.name)).toEqual(["adam", "Mel", "Zoe"])
  })

  it("is empty for an empty ledger", () => {
    expect(personTotals([])).toEqual([])
    expect(personTotals(undefined)).toEqual([])
  })
})

describe("personBreakdown", () => {
  const rows = [
    { ...evenExpense(A, 9000, [A, B]), _id: "x1", title: "Dinner" },
    { ...evenExpense(B, 3000, [B, C]), _id: "x2", title: "Cab" },
  ]

  it("shows what someone paid and what they owe, per expense", () => {
    const breakdown = personBreakdown(rows, A)

    expect(breakdown).toEqual([
      {
        expenseId: "x1",
        title: "Dinner",
        date: undefined,
        paidCents: 9000,
        shareCents: 4500,
      },
    ])
  })

  it("includes an expense someone shares but did not pay for", () => {
    const breakdown = personBreakdown(rows, B)

    expect(breakdown.map((r) => r.title)).toEqual(["Dinner", "Cab"])
    expect(breakdown[0]).toMatchObject({ paidCents: 0, shareCents: 4500 })
    expect(breakdown[1]).toMatchObject({ paidCents: 3000, shareCents: 1500 })
  })

  it("leaves out expenses that are none of their business", () => {
    // C was not on the dinner at all.
    expect(personBreakdown(rows, C).map((r) => r.title)).toEqual(["Cab"])
  })

  it("adds up to that person's row in the totals", () => {
    for (const person of personTotals(rows)) {
      const breakdown = personBreakdown(rows, person.userId)
      expect(breakdown.reduce((s, r) => s + r.paidCents, 0)).toBe(
        person.paidCents
      )
      expect(breakdown.reduce((s, r) => s + r.shareCents, 0)).toBe(
        person.shareCents
      )
    }
  })

  it("preserves ledger order", () => {
    expect(personBreakdown(rows, B).map((r) => r.expenseId)).toEqual([
      "x1",
      "x2",
    ])
  })

  it("is empty for somebody with no id, or nobody", () => {
    expect(personBreakdown(rows, null)).toEqual([])
    expect(personBreakdown(rows, "nobody")).toEqual([])
    expect(personBreakdown(undefined, A)).toEqual([])
  })
})

describe("settleUpSummary", () => {
  it("names both ends of every payment", () => {
    const { payments } = settleUpSummary([evenExpense(A, 5000, [A, B])])

    expect(payments).toEqual([
      { fromId: B, toId: A, amountCents: 2500, fromName: "BBB", toName: "AAA" },
    ])
  })

  it("totals the ledger and carries a row per person", () => {
    const summary = settleUpSummary([
      evenExpense(A, 9000, [A, B, C]),
      evenExpense(B, 3000, [A, B, C]),
    ])

    expect(summary.totalCents).toBe(12000)
    expect(summary.totals).toHaveLength(3)
    // The Total line under the Paid column is the ledger total.
    expect(summary.totals.reduce((s, t) => s + t.paidCents, 0)).toBe(
      summary.totalCents
    )
  })

  it("reports an empty ledger as square", () => {
    const summary = settleUpSummary([])

    expect(summary.payments).toEqual([])
    expect(summary.totalCents).toBe(0)
    expect(summary.totals).toEqual([])
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
