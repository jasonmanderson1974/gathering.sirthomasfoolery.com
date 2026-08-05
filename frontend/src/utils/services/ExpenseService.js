import { get, post, put, _delete } from "../fetch_utils"
import { serverURL } from "@/constants"

/**
 * The Settle Up tab on a gathering (F22): a shared expense ledger.
 *
 * Shared, unlike PersonalService's tabs — everyone who can see the gathering
 * reads the same rows. Which is why the permission rules live entirely on the
 * server: nothing here names a role, and the `canEdit` a client renders buttons
 * from is computed per-request rather than derived locally.
 *
 * Amounts are integer cents in every direction. There is no float on this path.
 */

/**
 * Fetch the whole ledger, newest first. Each row carries `canEdit` for the
 * calling user.
 * @returns {Promise<Array>} always an array, never null
 */
export const getExpenses = (eventId) => {
  return get(`/events/${eventId}/expenses`)
}

/**
 * The members an expense may be split between: everyone who took part in this
 * gathering and is a member or above, plus the caller. Guests are excluded and
 * are refused this endpoint outright, so only call it behind a member check.
 */
export const getExpenseParticipants = (eventId) => {
  return get(`/events/${eventId}/expenses/participants`)
}

/**
 * Add an expense.
 *
 * payload: {date, title, description, amountCents, paidBy, splitMode,
 *           participants[] | splits[]}
 *
 * `participants` drives an even split and `splits` carries typed per-person
 * amounts; which one is read is decided by `splitMode`. The server resolves the
 * even split itself and checks a by-amount split sums to the total exactly, so
 * a client never has to be trusted with the arithmetic — see expenseForm.js for
 * the preview that mirrors it.
 */
export const createExpense = (eventId, payload) => {
  return post(`/events/${eventId}/expenses`, payload)
}

/** Rewrite an expense. Same payload as createExpense. */
export const updateExpense = (eventId, expenseId, payload) => {
  return put(`/events/${eventId}/expenses/${expenseId}`, payload)
}

/**
 * Remove an expense from the ledger. A soft delete server-side — the row leaves
 * the ledger and the balances, but its change history is kept.
 */
export const deleteExpense = (eventId, expenseId) => {
  return _delete(`/events/${eventId}/expenses/${expenseId}`)
}

/**
 * Attach a receipt photo, as a base64 data URL. Downscale it first
 * (downscaleImageFile in expenseForm.js) — the server caps the request body,
 * and a straight-off-the-camera original will be refused.
 * @returns {Promise<{_id: string, width: number, height: number}>}
 */
export const uploadExpenseReceipt = (eventId, expenseId, image) => {
  return post(`/events/${eventId}/expenses/${expenseId}/receipts`, { image })
}

export const deleteExpenseReceipt = (eventId, expenseId, receiptId) => {
  return _delete(`/events/${eventId}/expenses/${expenseId}/receipts/${receiptId}`)
}

/**
 * The URL an <img> loads a receipt from.
 *
 * No cache-busting parameter, unlike avatarUrl: a receipt's bytes never change
 * — an edit uploads a new one under a new id — so the id alone identifies the
 * content, and the server serves it immutable.
 *
 * The route requires a session, so the same crossorigin caveat as
 * isOwnAvatarUrl applies under `npm run serve`, where the SPA and the API are
 * on different ports.
 */
export const expenseReceiptUrl = (eventId, expenseId, receiptId) =>
  `${serverURL}/events/${eventId}/expenses/${expenseId}/receipts/${receiptId}`
