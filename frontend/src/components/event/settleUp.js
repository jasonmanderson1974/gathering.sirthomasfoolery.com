/**
 * Turning a gathering's expense ledger into "who owes whom" (F22).
 *
 * Balances are DERIVED, never stored. Every expense records who paid and what
 * each person's share was, and that is enough — a stored balance would be a
 * second copy of the same fact, free to drift from the rows it came from the
 * first time an edit half-landed.
 *
 * Pure functions in a plain module rather than methods on the component,
 * because vitest here runs in node with no jsdom: nothing inside a .vue file is
 * coverable, and this is the arithmetic that most needs to be.
 *
 * Everything is integer cents. No float touches this path.
 */

/**
 * Each person's net position across the ledger, in cents.
 *
 * Positive means they are owed; negative means they owe. Paying for something
 * credits you the whole amount and debits you only your own share, which is
 * what makes the two sides net out.
 *
 * Sums to zero by construction, because the server guarantees each expense's
 * shares sum to its total. That invariant is the reason this can be a plain sum
 * with no rounding logic of its own.
 *
 * @param {Array} expenses rows as the API returns them
 * @returns {Map<string, number>} user id hex → cents
 */
export const netBalances = (expenses) => {
  const balances = new Map()

  const add = (id, cents) => {
    if (!id) return
    balances.set(id, (balances.get(id) ?? 0) + cents)
  }

  for (const expense of expenses ?? []) {
    add(expense.paidBy, expense.amountCents ?? 0)
    for (const split of expense.splits ?? []) {
      add(split.userId, -(split.amountCents ?? 0))
    }
  }

  // Someone who paid exactly their own share nets to zero and is simply not
  // part of the settlement — dropping them here keeps them out of every
  // downstream count.
  for (const [id, cents] of balances) {
    if (cents === 0) balances.delete(id)
  }

  return balances
}

/**
 * The display name for each account the ledger mentions, taken from the
 * snapshots on the rows themselves.
 *
 * The server re-resolves those snapshots against the live accounts on every
 * read, so they are current wherever the account still exists and are the
 * account's last known name where it doesn't. Either way there is nothing extra
 * to fetch.
 *
 * @returns {Map<string, string>} user id hex → name
 */
export const namesFromExpenses = (expenses) => {
  const names = new Map()

  const remember = (id, name) => {
    if (id && name) names.set(id, name)
  }

  for (const expense of expenses ?? []) {
    remember(expense.paidBy, expense.paidByName)
    remember(expense.createdBy, expense.createdByName)
    for (const split of expense.splits ?? []) {
      remember(split.userId, split.name)
    }
  }

  return names
}

/**
 * The shortest set of payments that clears a set of balances.
 *
 * Greedy: walk the largest debtor against the largest creditor and transfer
 * whichever of the two is smaller. Each transfer zeroes at least one of the
 * pair, so the whole thing terminates in at most (people - 1) payments — which
 * is the "reduce the overall number of transactions" the feature asks for. Four
 * people who each owe the next one round get two payments, not four.
 *
 * This is not guaranteed to be the theoretical minimum (that problem is
 * NP-hard), but it is within one of it for any group this app will ever see,
 * and it is O(n log n) rather than exponential.
 *
 * Ties are broken by user id so the output is stable between renders. A
 * settlement list that reshuffles itself every time the page refreshes reads as
 * though something changed.
 *
 * @param {Map<string, number>} balances from netBalances
 * @returns {Array<{fromId: string, toId: string, amountCents: number}>}
 */
export const simplifyDebts = (balances) => {
  const creditors = []
  const debtors = []

  for (const [id, cents] of balances ?? new Map()) {
    if (cents > 0) creditors.push({ id, cents })
    else if (cents < 0) debtors.push({ id, cents: -cents })
  }

  // Largest first, then by id — the second term is what makes this
  // deterministic when two people are owed the same amount.
  const largestFirst = (a, b) =>
    b.cents - a.cents || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0)
  creditors.sort(largestFirst)
  debtors.sort(largestFirst)

  const payments = []
  let owing = 0
  let owed = 0

  while (owing < debtors.length && owed < creditors.length) {
    const amountCents = Math.min(debtors[owing].cents, creditors[owed].cents)
    if (amountCents > 0) {
      payments.push({
        fromId: debtors[owing].id,
        toId: creditors[owed].id,
        amountCents,
      })
    }

    debtors[owing].cents -= amountCents
    creditors[owed].cents -= amountCents
    if (debtors[owing].cents === 0) owing++
    if (creditors[owed].cents === 0) owed++
  }

  return payments
}

/**
 * What each person put in, and what each person is on the hook for.
 *
 * Two different questions, and the gap between them is the whole point of the
 * feature: paying for dinner does not make it yours, and being in the split does
 * not mean you paid. `netCents` is the difference — positive means owed,
 * negative means owing — and it is the same number `netBalances` produces, from
 * the same rows.
 *
 * Both columns sum to the ledger total, because every expense has exactly one
 * payer and its shares always sum to its amount. That is what lets the panel
 * print one "Total" line under either.
 *
 * Sorted by name, not by amount: alphabetical order is stable as expenses are
 * added, so a row does not jump position because somebody bought a round. It
 * matches how the participants picker is sorted server-side, too.
 *
 * @returns {Array<{userId, name, paidCents, shareCents, netCents}>}
 */
export const personTotals = (expenses) => {
  const rows = expenses ?? []
  const totals = new Map()

  const bump = (id, field, cents) => {
    if (!id) return
    const entry = totals.get(id) ?? { paidCents: 0, shareCents: 0 }
    entry[field] += cents
    totals.set(id, entry)
  }

  for (const expense of rows) {
    bump(expense.paidBy, "paidCents", expense.amountCents ?? 0)
    for (const split of expense.splits ?? []) {
      bump(split.userId, "shareCents", split.amountCents ?? 0)
    }
  }

  const names = namesFromExpenses(rows)

  return [...totals]
    .map(([userId, entry]) => ({
      userId,
      name: names.get(userId) ?? "Someone",
      paidCents: entry.paidCents,
      shareCents: entry.shareCents,
      netCents: entry.paidCents - entry.shareCents,
    }))
    .sort(
      (a, b) =>
        a.name.localeCompare(b.name, undefined, { sensitivity: "base" }) ||
        (a.userId < b.userId ? -1 : a.userId > b.userId ? 1 : 0)
    )
}

/**
 * One person's costs, expense by expense — what the hover panel shows.
 *
 * An expense appears if the person paid for it, shares it, or both; a row where
 * they did neither is not their business and is left out. Ledger order is
 * preserved (newest first, as the API returns it) so the breakdown reads in the
 * same order as the list beside it.
 *
 * @returns {Array<{expenseId, title, date, paidCents, shareCents}>}
 */
export const personBreakdown = (expenses, userId) => {
  const rows = []
  if (!userId) return rows

  for (const expense of expenses ?? []) {
    const paidCents = expense.paidBy === userId ? expense.amountCents ?? 0 : 0
    const split = (expense.splits ?? []).find((s) => s.userId === userId)
    const shareCents = split ? split.amountCents ?? 0 : 0

    if (paidCents || shareCents) {
      rows.push({
        expenseId: expense._id,
        title: expense.title,
        date: expense.date,
        paidCents,
        shareCents,
      })
    }
  }

  return rows
}

/**
 * Everything the summary panel needs, in one pass over the ledger: what each
 * person paid, the ledger total, and the payments that would settle it.
 *
 * @returns {{totals: Array, payments: Array, totalCents: number}}
 *   payments carry fromName/toName resolved from the ledger's own snapshots.
 */
export const settleUpSummary = (expenses) => {
  const rows = expenses ?? []
  const balances = netBalances(rows)
  const names = namesFromExpenses(rows)

  const name = (id) => names.get(id) ?? "Someone"

  const payments = simplifyDebts(balances).map((payment) => ({
    ...payment,
    fromName: name(payment.fromId),
    toName: name(payment.toId),
  }))

  return {
    totals: personTotals(rows),
    payments,
    totalCents: rows.reduce(
      (sum, expense) => sum + (expense.amountCents ?? 0),
      0
    ),
  }
}
