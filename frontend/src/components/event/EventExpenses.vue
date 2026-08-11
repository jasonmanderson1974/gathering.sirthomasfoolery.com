<template>
  <div>
    <!-- The balances, inline. Gated so they render in exactly one place: above
         1024px they live in the right-hand sidebar, which is visible while this
         tab is open, and showing the same figures twice reads as two different
         answers. Below that there is no sidebar, so they come back here. -->
    <SettleUpSummary v-if="!hasSidebar" :expenses="expenses" />

    <div
      class="tw-mt-3 tw-rounded-md tw-border tw-border-brass-dim tw-bg-leather tw-p-3 tw-text-parchment sm:tw-p-4"
    >
      <div class="tw-mb-2 tw-flex tw-items-center tw-justify-between tw-gap-2">
        <div class="tw-text-base tw-font-medium">Expenses</div>
        <div class="tw-flex tw-flex-none tw-items-center tw-gap-1">
          <v-progress-circular
            v-if="refreshing"
            indeterminate
            color="brass"
            size="16"
            width="2"
          />
          <v-btn
            v-if="canAdd"
            size="small"
            variant="outlined"
            class="tw-text-brass"
            @click="openNew"
          >
            <v-icon size="small" start>mdi-plus</v-icon>
            Add expense
          </v-btn>
        </div>
      </div>

      <!-- What each person has put in, at the top of the list they are the sum
           of. The same figures as the summary panel, which on a wide screen is
           off in the sidebar — this is the column you are actually reading when
           you want to know whether a number looks right. -->
      <div
        v-if="expenses.length"
        class="tw-mb-3 tw-flex tw-flex-wrap tw-items-baseline tw-gap-x-4 tw-gap-y-1 tw-border-b tw-border-brass-dim/60 tw-pb-2 tw-text-sm"
      >
        <span
          v-for="person in totals"
          :key="person.userId"
          class="tw-text-parchment-dim"
        >
          {{ person.name }}
          <span class="tw-tabular-nums tw-text-parchment">
            {{ formatCents(person.paidCents) }}
          </span>
        </span>
        <span class="tw-ml-auto tw-font-medium">
          Total
          <span class="tw-tabular-nums tw-text-brass">
            {{ formatCents(totalCents) }}
          </span>
        </span>
      </div>

      <div v-if="!expenses.length" class="tw-text-sm tw-text-parchment-dim">
        {{
          canAdd
            ? "No expenses yet. Add the first one and it will be split with whoever was there."
            : "No expenses yet."
        }}
      </div>

      <div v-else class="tw-space-y-2">
        <div
          v-for="expense in expenses"
          :key="expense._id"
          class="tw-rounded tw-border tw-border-brass-dim/60 tw-p-2 sm:tw-p-3"
        >
          <div class="tw-flex tw-items-start tw-justify-between tw-gap-2">
            <div class="tw-min-w-0">
              <div class="tw-break-words tw-font-medium">
                {{ expense.title }}
              </div>
              <div class="tw-text-xs tw-text-parchment-dim">
                {{ formatExpenseDate(expense.date) }} ·
                {{ expense.paidByName }} paid ·
                {{ splitLabel(expense) }}
              </div>
            </div>
            <div class="tw-flex tw-flex-none tw-items-center tw-gap-1">
              <span class="tw-text-brass">{{
                formatCents(expense.amountCents)
              }}</span>
              <v-btn
                v-if="expense.canEdit"
                icon
                size="x-small"
                class="tw-text-parchment-dim"
                title="Edit expense"
                @click="openEdit(expense)"
              >
                <v-icon size="small">mdi-pencil</v-icon>
              </v-btn>
              <v-btn
                v-if="expense.canEdit"
                icon
                size="x-small"
                class="tw-text-red"
                title="Delete expense"
                @click="askDelete(expense)"
              >
                <v-icon size="small">mdi-delete</v-icon>
              </v-btn>
            </div>
          </div>

          <!-- Receipt thumbnails. crossorigin is load-bearing under
               `npm run serve`, where the SPA is on :8080 and the API on :3002:
               the serving route needs the session cookie, and a plain <img>
               sends none. Same reasoning as isOwnAvatarUrl. -->
          <div
            v-if="expense.receipts?.length"
            class="tw-mt-2 tw-flex tw-flex-wrap tw-gap-2"
          >
            <div
              v-for="receipt in expense.receipts"
              :key="receipt._id"
              class="tw-relative"
            >
              <a
                :href="receiptUrl(expense, receipt)"
                target="_blank"
                rel="noopener"
                title="Open the full receipt"
              >
                <img
                  :src="receiptUrl(expense, receipt)"
                  crossorigin="use-credentials"
                  class="tw-h-16 tw-w-16 tw-rounded tw-border tw-border-brass-dim tw-object-cover"
                />
              </a>
              <v-btn
                v-if="expense.canEdit"
                icon
                size="x-small"
                class="tw-absolute tw-right-0 tw-top-0 tw-bg-wood-deep/80 tw-text-parchment"
                title="Remove receipt"
                @click="askDeleteReceipt(expense, receipt)"
              >
                <v-icon size="x-small">mdi-close</v-icon>
              </v-btn>
            </div>
          </div>

          <div
            v-if="expense.description"
            class="tw-mt-2 tw-break-words tw-text-sm"
          >
            {{ expense.description }}
          </div>

          <!-- Who owes what on this one row, and how it got to be this way.
               Both behind a disclosure: the ledger is scanned far more often
               than it is interrogated. -->
          <div class="tw-mt-2 tw-flex tw-flex-wrap tw-gap-3">
            <a
              class="tw-text-xs tw-text-brass"
              @click="toggle(expense._id, 'split')"
            >
              {{ isOpen(expense._id, "split") ? "Hide split" : "Show split" }}
            </a>
            <a
              v-if="expense.history?.length"
              class="tw-text-xs tw-text-parchment-dim"
              @click="toggle(expense._id, 'history')"
            >
              {{ isOpen(expense._id, "history") ? "Hide history" : "History" }}
            </a>
          </div>

          <div
            v-if="isOpen(expense._id, 'split')"
            class="tw-mt-1 tw-space-y-0.5 tw-text-xs"
          >
            <div
              v-for="split in expense.splits"
              :key="split.userId"
              class="tw-flex tw-justify-between tw-gap-2"
            >
              <span class="tw-text-parchment-dim">{{ split.name }}</span>
              <span>{{ formatCents(split.amountCents) }}</span>
            </div>
          </div>

          <div
            v-if="isOpen(expense._id, 'history')"
            class="tw-mt-1 tw-space-y-1 tw-text-xs tw-text-parchment-dim"
          >
            <div v-for="(change, i) in expense.history" :key="i">
              <span class="tw-font-medium">{{ change.byName }}</span>
              {{ actionLabel(change.action) }}
              <span>{{ formatExpenseDate(change.at) }}</span>
              <div v-if="change.changes?.length" class="tw-pl-3">
                <div v-for="(field, j) in change.changes" :key="j">
                  {{ fieldLabel(field.field) }}:
                  <span class="tw-line-through">{{ field.from || "—" }}</span>
                  →
                  <span class="tw-text-parchment">{{ field.to || "—" }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <ExpenseDialog
      ref="dialog"
      v-model="dialog"
      :event-id="eventId"
      :expense="editing"
      @save="onSave"
    />

    <ConfirmDeleteDialog
      :model-value="!!pendingDelete"
      :title="pendingDelete ? pendingDelete.title : ''"
      :body="pendingDelete ? pendingDelete.body : ''"
      @update:model-value="(open) => !open && (pendingDelete = null)"
      @confirm="confirmDelete"
    />
  </div>
</template>

<script>
import { mapGetters } from "vuex"
import SettleUpSummary from "@/components/event/SettleUpSummary.vue"
import ExpenseDialog from "@/components/event/ExpenseDialog.vue"
import ConfirmDeleteDialog from "@/components/general/ConfirmDeleteDialog.vue"
import { personTotals } from "@/components/event/settleUp"
import { formatCents, formatExpenseDate } from "@/components/event/expenseForm"
import { expenseReceiptUrl } from "@/utils/services/ExpenseService"

const ACTION_LABELS = Object.freeze({
  created: "added this",
  edited: "edited it",
  deleted: "deleted it",
  "receipt-added": "added a receipt",
  "receipt-removed": "removed a receipt",
})

const FIELD_LABELS = Object.freeze({
  amount: "Amount",
  title: "Title",
  date: "Date",
  description: "Notes",
  paidBy: "Paid by",
  split: "Split",
})

/**
 * The "Settle Up" tab: a gathering's shared expense ledger (F22).
 *
 * Renders and emits; it never writes. The rows live on Event.vue (via
 * expensesMixin) because the sidebar summary needs the same data without this
 * tab ever having been opened — the same reason the shared lists are owned up
 * there rather than by their panel, and the opposite of My Lists / My Notes,
 * which have exactly one consumer and fetch for themselves.
 *
 * Every permission shown here comes from the server's per-row `canEdit`. The
 * one local check is `canAdd`, which only decides whether a button is drawn;
 * the server refuses a guest's POST regardless.
 */
export default {
  name: "EventExpenses",

  components: { SettleUpSummary, ExpenseDialog, ConfirmDeleteDialog },

  props: {
    /** Short id or _id — whichever the route gave; the server resolves both. */
    eventId: { type: String, required: true },
    expenses: { type: Array, default: () => [] },
    refreshing: { type: Boolean, default: false },
    /**
     * Whether the content column's right-hand strip is showing the balances.
     * When it is, this panel must not show them too.
     */
    hasSidebar: { type: Boolean, default: false },
  },

  emits: ["create-expense", "edit-expense", "delete-expense", "delete-receipt"],

  data: () => ({
    dialog: false,
    editing: null,
    // Which disclosures are open, keyed "<expenseId>:<section>". A plain Set
    // would not be reactive in Vue 2.
    open: {},
    // Holds both the wording and the action, so ConfirmDeleteDialog can stay
    // stateless — the idiom EventLists.vue uses.
    pendingDelete: null,
  }),

  computed: {
    // Guests read the ledger but never write to it.
    ...mapGetters({ canAdd: "canSeeMembersOnly" }),

    /** What each person has paid — the same figures the summary panel shows. */
    totals() {
      return personTotals(this.expenses)
    },

    totalCents() {
      return this.expenses.reduce(
        (sum, expense) => sum + (expense.amountCents ?? 0),
        0
      )
    },
  },

  methods: {
    formatCents,
    formatExpenseDate,

    splitLabel(expense) {
      const n = expense.splits?.length ?? 0
      return `split ${n} ${n === 1 ? "way" : "ways"}`
    },

    actionLabel(action) {
      return ACTION_LABELS[action] ?? "changed it"
    },

    fieldLabel(field) {
      return FIELD_LABELS[field] ?? field
    },

    receiptUrl(expense, receipt) {
      return expenseReceiptUrl(this.eventId, expense._id, receipt._id)
    },

    isOpen(expenseId, section) {
      return !!this.open[`${expenseId}:${section}`]
    },

    toggle(expenseId, section) {
      const key = `${expenseId}:${section}`
      this.open = { ...this.open, [key]: !this.open[key] }
    },

    openNew() {
      this.editing = null
      this.dialog = true
    },

    openEdit(expense) {
      this.editing = expense
      this.dialog = true
    },

    /**
     * The dialog hands up a finished payload and any queued photos; the write
     * happens above us. It stays open until we say otherwise, so a failure
     * leaves what was typed on screen rather than discarding it.
     */
    onSave({ payload, photos }) {
      const event = this.editing ? "edit-expense" : "create-expense"
      const detail = this.editing
        ? { expenseId: this.editing._id, payload, photos }
        : { payload, photos }

      this.$emit(event, {
        ...detail,
        done: (ok) => this.$refs.dialog?.finish(ok),
      })
    },

    askDelete(expense) {
      this.pendingDelete = {
        title: `Delete "${expense.title}"?`,
        body: "It will stop counting towards who owes whom.",
        run: () => this.$emit("delete-expense", expense._id),
      }
    },

    askDeleteReceipt(expense, receipt) {
      this.pendingDelete = {
        title: "Remove this receipt?",
        body: `The photo will be deleted from "${expense.title}".`,
        run: () =>
          this.$emit("delete-receipt", {
            expenseId: expense._id,
            receiptId: receipt._id,
          }),
      }
    },

    confirmDelete() {
      const pending = this.pendingDelete
      this.pendingDelete = null
      pending?.run()
    },
  },
}
</script>
