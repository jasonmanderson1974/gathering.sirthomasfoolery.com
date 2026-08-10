import { mapActions } from "vuex"
import {
  getExpenses,
  createExpense,
  updateExpense,
  deleteExpense,
  uploadExpenseReceipt,
  deleteExpenseReceipt,
} from "@/utils/services/ExpenseService"

/**
 * The Settle Up ledger's state and writes, for the Event view (F22).
 *
 * The rows live on Event.vue rather than inside EventExpenses.vue because TWO
 * children need them: the tab, and the sidebar's who-owes-whom summary, which
 * has to be right without the tab ever having been opened. That is the same
 * arrangement the shared lists use, and the opposite of My Lists / My Notes,
 * which have exactly one consumer each and fetch for themselves.
 *
 * Kept as a mixin rather than another 150 lines in a view that is already
 * 1,300, following pluginMessagesMixin. Mixin methods run against the component
 * instance, so `this.event`, `this.showError` and the rest resolve exactly as
 * they would inline.
 *
 * Every handler PERSISTS THEN REFETCHES rather than mutating the local array.
 * Nothing here is optimistic: the server resolves each split itself, so the
 * shares it stores are the only ones worth rendering, and guessing at them
 * locally would show numbers that quietly differ from the truth.
 */
export default {
  data: () => ({
    expenses: [],
    refreshingExpenses: false,
  }),

  methods: {
    ...mapActions(["showError"]),

    /** Short id where there is one, matching bandEventId. */
    expenseEventId() {
      return this.event?.shortId ?? this.event?._id ?? ""
    },

    /**
     * Re-read the ledger.
     *
     * Busy-guarded like refreshLists: selecting the tab and saving an expense
     * can both land within a tick of each other, and two overlapping reads
     * would race to assign the array.
     */
    async refreshExpenses() {
      const id = this.expenseEventId()
      if (!id || this.refreshingExpenses) return

      this.refreshingExpenses = true
      try {
        this.expenses = (await getExpenses(id)) ?? []
      } catch (err) {
        this.showError("Could not load the expenses. Please try again.")
      } finally {
        this.refreshingExpenses = false
      }
    },

    /**
     * Upload whatever photos were queued alongside a new or edited expense.
     *
     * Sequential, not parallel: the server caps the number of receipts per
     * expense and counts against the collection, so concurrent uploads of the
     * last two slots would race and one would be rejected for no reason the
     * person could see.
     *
     * A failure here does NOT fail the expense — the row is already saved, and
     * telling someone their expense did not save because a photo did not would
     * be false.
     */
    async uploadReceipts(expenseId, photos) {
      let failed = 0
      for (const image of photos ?? []) {
        try {
          await uploadExpenseReceipt(this.expenseEventId(), expenseId, image)
        } catch (err) {
          failed++
        }
      }
      if (failed) {
        this.showError(
          failed === 1
            ? "The expense saved, but one receipt photo did not."
            : `The expense saved, but ${failed} receipt photos did not.`
        )
      }
    },

    /**
     * `done(true)` closes the dialog; `done(false)` leaves it open with what was
     * typed still in it, which is what a failure wants — the alternative is
     * discarding an entry someone has just filled in.
     */
    async onCreateExpense({ payload, photos, done }) {
      try {
        const created = await createExpense(this.expenseEventId(), payload)
        await this.uploadReceipts(created?._id, photos)
        done?.(true)
      } catch (err) {
        this.showError(this.expenseErrorMessage(err))
        done?.(false)
      }
      await this.refreshExpenses()
    },

    async onEditExpense({ expenseId, payload, photos, done }) {
      try {
        await updateExpense(this.expenseEventId(), expenseId, payload)
        await this.uploadReceipts(expenseId, photos)
        done?.(true)
      } catch (err) {
        this.showError(this.expenseErrorMessage(err))
        done?.(false)
      }
      await this.refreshExpenses()
    },

    async onDeleteExpense(expenseId) {
      try {
        await deleteExpense(this.expenseEventId(), expenseId)
      } catch (err) {
        this.showError("Could not delete that expense. Please try again.")
      }
      await this.refreshExpenses()
    },

    async onDeleteReceipt({ expenseId, receiptId }) {
      try {
        await deleteExpenseReceipt(this.expenseEventId(), expenseId, receiptId)
      } catch (err) {
        this.showError("Could not remove that receipt. Please try again.")
      }
      await this.refreshExpenses()
    },

    /**
     * Turn the server's error code into something worth reading.
     *
     * The validation codes all describe something the person can fix, and
     * saying which one is the difference between correcting a typo and trying
     * the same thing again. Anything unrecognised falls back to the generic
     * wording rather than leaking a code.
     */
    expenseErrorMessage(err) {
      switch (err?.error) {
        case "split-mismatch":
          return "The shares don't add up to the total. Please check them."
        case "invalid-amount":
          return "That amount doesn't look right."
        case "invalid-title":
          return "Please give the expense a name."
        case "invalid-date":
          // The picker is bounded to the same window, so reaching this means a
          // hand-rolled client — but the generic wording would send someone
          // round the same failing save forever.
          return "That date is too far from today."
        case "no-participants":
          return "Choose at least one person to split this with."
        case "not-a-participant":
          return "Someone in that split isn't a member of this gathering."
        case "no-changes":
          return "Nothing was changed."
        case "not-authorized":
          return "You can only edit expenses you entered."
        default:
          return "Could not save that expense. Please try again."
      }
    },
  },
}
