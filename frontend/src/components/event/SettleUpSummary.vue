<template>
  <div
    class="tw-mt-3 tw-rounded-md tw-border tw-border-brass-dim tw-bg-leather tw-p-3 tw-text-parchment sm:tw-p-4"
  >
    <div class="tw-mb-2 tw-flex tw-items-baseline tw-justify-between tw-gap-2">
      <span class="tw-text-base tw-font-medium">Settle Up</span>
      <span v-if="summary.totalCents" class="tw-text-xs tw-text-parchment-dim">
        {{ formatCents(summary.totalCents) }} in all
      </span>
    </div>

    <div v-if="!expenses.length" class="tw-text-sm tw-text-parchment-dim">
      Nothing spent yet.
    </div>

    <div v-else-if="!summary.payments.length" class="tw-text-sm tw-text-parchment-dim">
      All square — nobody owes anybody.
    </div>

    <div v-else class="tw-space-y-1.5">
      <div
        v-for="payment in summary.payments"
        :key="`${payment.fromId}-${payment.toId}`"
        class="tw-flex tw-items-center tw-gap-2 tw-text-sm"
      >
        <span class="tw-min-w-0 tw-flex-grow tw-break-words">
          <span class="tw-font-medium">{{ payment.fromName }}</span>
          <v-icon x-small class="tw-mx-1 tw-text-parchment-dim">
            mdi-arrow-right
          </v-icon>
          <span class="tw-font-medium">{{ payment.toName }}</span>
        </span>
        <span class="tw-flex-none tw-text-brass">
          {{ formatCents(payment.amountCents) }}
        </span>
      </div>

      <div class="tw-pt-1 tw-text-xs tw-text-parchment-dim">
        {{ paymentCountLabel }} to square everyone up.
      </div>
    </div>
  </div>
</template>

<script>
import { settleUpSummary } from "@/components/event/settleUp"
import { formatCents } from "@/components/event/expenseForm"

/**
 * "Who owes whom", reduced to the fewest payments that would clear it (F22).
 *
 * Derived entirely from the expense rows it is handed — no fetch of its own,
 * and nothing stored anywhere. That is what lets it sit in the sidebar and stay
 * in step with the ledger below without either one having to tell the other
 * anything.
 *
 * The arithmetic is in settleUp.js, where it is unit-tested; this file only
 * renders the result.
 */
export default {
  name: "SettleUpSummary",

  props: {
    expenses: { type: Array, default: () => [] },
  },

  computed: {
    summary() {
      return settleUpSummary(this.expenses)
    },

    paymentCountLabel() {
      const n = this.summary.payments.length
      return n === 1 ? "One payment" : `${n} payments`
    },
  },

  methods: { formatCents },
}
</script>
