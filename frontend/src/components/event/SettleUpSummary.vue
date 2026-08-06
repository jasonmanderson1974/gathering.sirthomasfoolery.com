<template>
  <div
    class="tw-mt-3 tw-rounded-md tw-border tw-border-brass-dim tw-bg-leather tw-p-3 tw-text-parchment sm:tw-p-4"
  >
    <div class="tw-mb-2 tw-text-base tw-font-medium">Settle Up</div>

    <div v-if="!expenses.length" class="tw-text-sm tw-text-parchment-dim">
      Nothing spent yet.
    </div>

    <template v-else>
      <!-- What each person put in, with their breakdown opening beneath the row
           on hover.

           Expanded in place rather than in a floating v-menu, for two reasons.
           A panel floating over an 18rem sidebar covers the very rows you are
           comparing against; and hover alone is unreachable on a phone, where
           this same panel renders inline in the tab and nothing hovers — so the
           row is clickable too, and a tap does the same thing.

           The breakdown opens BELOW its row, so the row under the cursor never
           moves and there is no hover/unhover flicker loop. -->
      <div v-for="person in summary.totals" :key="person.userId">
        <div
          class="tw-flex tw-cursor-pointer tw-items-baseline tw-justify-between tw-gap-2 tw-rounded tw-px-1 tw-py-0.5 tw-text-sm hover:tw-bg-brass/10"
          :class="isOpen(person) ? 'tw-bg-brass/10' : ''"
          @mouseenter="open = person.userId"
          @mouseleave="open = null"
          @click="toggle(person.userId)"
        >
          <span class="tw-min-w-0 tw-truncate">{{ person.name }}</span>
          <span class="tw-flex-none tw-tabular-nums">
            {{ formatCents(person.paidCents) }}
          </span>
        </div>

        <!-- v-if, not v-show: `tw-space-y-*` and the flex rows inside compile
             to `!important` under this project's Tailwind config, which beats
             the inline display:none v-show sets. See CLAUDE.md. -->
        <div
          v-if="isOpen(person)"
          class="tw-mb-1 tw-ml-1 tw-border-l tw-border-brass-dim tw-pl-2 tw-text-xs"
          @mouseenter="open = person.userId"
          @mouseleave="open = null"
        >
          <div class="tw-flex tw-justify-between tw-gap-3 tw-text-parchment-dim">
            <span>Responsible for</span>
            <span class="tw-tabular-nums">{{ formatCents(person.shareCents) }}</span>
          </div>
          <div class="tw-flex tw-justify-between tw-gap-3">
            <span class="tw-text-parchment-dim">{{ netLabel(person) }}</span>
            <span :class="netClass(person)">{{ netAmount(person) }}</span>
          </div>

          <div class="tw-mt-1 tw-space-y-0.5 tw-border-t tw-border-brass-dim/50 tw-pt-1">
            <!-- Title on its own line with the figures right-aligned beneath,
                 rather than the two sharing a row. Side by side, an expense
                 someone both paid for and shares needs ~24 characters of
                 figures, which left so little for the title in an 18rem sidebar
                 that "Cabin deposit" truncated to "Cabin d…". Only one person's
                 breakdown is open at a time, so the extra height is cheap. -->
            <div
              v-for="row in breakdownFor(person.userId)"
              :key="row.expenseId"
              class="tw-py-0.5"
            >
              <div class="tw-truncate tw-text-parchment-dim">{{ row.title }}</div>
              <div class="tw-flex tw-justify-end tw-gap-2 tw-tabular-nums">
                <!-- "paid" only when they actually fronted this one, so a row
                     they merely share never reads as money they put in. -->
                <span v-if="row.paidCents" class="tw-text-brass">
                  paid {{ formatCents(row.paidCents) }}
                </span>
                <span v-if="row.shareCents">
                  share {{ formatCents(row.shareCents) }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div
        class="tw-mt-1 tw-flex tw-items-baseline tw-justify-between tw-gap-2 tw-border-t tw-border-brass-dim tw-px-1 tw-pt-1 tw-text-sm tw-font-medium"
      >
        <span>Total</span>
        <span class="tw-flex-none tw-tabular-nums">
          {{ formatCents(summary.totalCents) }}
        </span>
      </div>

      <div
        v-if="!summary.payments.length"
        class="tw-mt-3 tw-text-sm tw-text-parchment-dim"
      >
        All square — nobody owes anybody.
      </div>

      <div v-else class="tw-mt-3 tw-space-y-1.5">
        <div
          v-for="payment in summary.payments"
          :key="`${payment.fromId}-${payment.toId}`"
          class="tw-flex tw-items-center tw-gap-2 tw-text-sm"
        >
          <span class="tw-min-w-0 tw-flex-grow tw-break-words">
            <span class="tw-font-medium">{{ payment.fromName }}</span>
            <span class="tw-text-parchment-dim"> pays </span>
            <span class="tw-font-medium">{{ payment.toName }}</span>
          </span>
          <span class="tw-flex-none tw-tabular-nums tw-text-brass">
            {{ formatCents(payment.amountCents) }}
          </span>
        </div>

        <div class="tw-pt-1 tw-text-xs tw-text-parchment-dim">
          {{ paymentCountLabel }} to square everyone up.
        </div>
      </div>
    </template>
  </div>
</template>

<script>
import {
  settleUpSummary,
  personBreakdown,
} from "@/components/event/settleUp"
import { formatCents } from "@/components/event/expenseForm"

/**
 * What each person put in, and who owes whom — reduced to the fewest payments
 * that would clear it (F22).
 *
 * Derived entirely from the expense rows it is handed: no fetch of its own, and
 * nothing stored anywhere. That is what lets it sit in the sidebar and stay in
 * step with the ledger below without either one telling the other anything.
 *
 * The arithmetic is all in settleUp.js, where it is unit-tested; this file only
 * renders the result.
 */
export default {
  name: "SettleUpSummary",

  props: {
    expenses: { type: Array, default: () => [] },
  },

  data: () => ({
    /**
     * Whose breakdown is showing, by user id. One at a time: this is a glance
     * at one person's costs, not a table to read all of at once, and in a
     * narrow column several open at once would push the settlement off screen.
     */
    open: null,
  }),

  computed: {
    summary() {
      return settleUpSummary(this.expenses)
    },

    paymentCountLabel() {
      const n = this.summary.payments.length
      return n === 1 ? "One payment" : `${n} payments`
    },
  },

  methods: {
    formatCents,

    isOpen(person) {
      return this.open === person.userId
    },

    /**
     * Tap to pin a breakdown open, tap again to close.
     *
     * On a phone the click is the only way in — there is no hover — and on a
     * desktop it lets someone hold one open while reading the expenses beside
     * it, instead of it vanishing the moment the pointer leaves.
     */
    toggle(userId) {
      this.open = this.open === userId ? null : userId
    },

    breakdownFor(userId) {
      return personBreakdown(this.expenses, userId)
    },

    /**
     * "Owed" / "Owes" / "Square" — stated from that person's point of view,
     * because the row is theirs. A positive net means the gathering owes them.
     */
    netLabel(person) {
      if (person.netCents > 0) return "Owed"
      if (person.netCents < 0) return "Owes"
      return "Square"
    },

    netAmount(person) {
      // The sign is carried by the label, so the figure itself is never
      // negative — "Owes -$38.75" reads as a double negative.
      return person.netCents === 0
        ? "—"
        : formatCents(Math.abs(person.netCents))
    },

    netClass(person) {
      if (person.netCents > 0) return "tw-tabular-nums tw-text-brass"
      if (person.netCents < 0) return "tw-tabular-nums tw-text-parchment"
      return "tw-tabular-nums tw-text-parchment-dim"
    },
  },
}
</script>
