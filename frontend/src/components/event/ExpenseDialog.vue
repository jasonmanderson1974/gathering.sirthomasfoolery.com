<template>
  <v-dialog
    :model-value="modelValue"
    max-width="560"
    scrollable
    content-class="tw-m-0"
    @update:model-value="onDialogInput"
  >
    <v-card>
      <v-card-title class="tw-text-base">
        {{ expense ? "Edit expense" : "Add expense" }}
      </v-card-title>

      <v-card-text class="tw-pb-2">
        <!-- Date. A menu-wrapped picker rather than a text field: the day is
             almost always today or a day or two back, which is one tap here and
             typing a format nobody remembers otherwise. -->
        <v-menu
          v-model="dateMenu"
          :close-on-content-click="false"
          offset-y
          min-width="auto"
        >
          <template #activator="{ props }">
            <v-text-field
              :model-value="dateLabel"
              label="Date"
              readonly
              dense
              hide-details
              prepend-inner-icon="mdi-calendar"
              class="tw-mb-3"
              v-bind="props"
            />
          </template>
          <!--
            min/max mirror the server's expenseDateWindow (J10). Without them
            the picker happily navigates to a year the API rejects, and the
            person gets a save failure with no hint that the date is the reason.
          -->
          <v-date-picker
            v-model="date"
            :min="dateMin"
            :max="dateMax"
            no-title
            @input="dateMenu = false"
          />
        </v-menu>

        <v-text-field
          v-model="title"
          label="What was it?"
          placeholder="Dinner at the club"
          dense
          hide-details
          :maxlength="titleMaxLength"
          class="tw-mb-3"
        />

        <v-textarea
          v-model="description"
          label="Notes (optional)"
          dense
          hide-details
          auto-grow
          rows="2"
          :maxlength="descriptionMaxLength"
          class="tw-mb-3"
        />

        <!-- inputmode="decimal", NOT type="number": a number field brings
             spinners nobody wants on money, silently accepts exponent
             notation, and hands back a locale-dependent string anyway.
             parseAmount does the work. -->
        <v-text-field
          v-model="amount"
          label="Amount"
          prefix="$"
          inputmode="decimal"
          placeholder="0.00"
          dense
          :hide-details="!amountError"
          :error-messages="amountError"
          class="tw-mb-3"
        />

        <v-select
          v-model="paidBy"
          :items="participantOptions"
          item-title="name"
          item-value="id"
          label="Paid by"
          dense
          hide-details
          class="tw-mb-4"
        />

        <div class="tw-mb-1 tw-text-sm tw-font-medium">Split between</div>

        <div v-if="loadingParticipants" class="tw-py-4 tw-text-center">
          <v-progress-circular indeterminate color="brass" size="20" />
        </div>

        <template v-else>
          <div
            v-if="!participants.length"
            class="tw-text-sm tw-text-parchment-dim"
          >
            Nobody has taken part in this gathering yet.
          </div>

          <template v-else>
            <!-- v-if, not v-show: `tw-flex` compiles to display:flex !important
                 under this project's Tailwind config (important: true) and beats
                 the inline display:none v-show sets, so a hidden row would still
                 render. Same trap noted in CLAUDE.md. -->
            <div class="tw-mb-2 tw-flex tw-items-center tw-gap-4">
              <v-radio-group
                v-model="splitMode"
                row
                dense
                hide-details
                class="tw-mt-0 tw-pt-0"
              >
                <v-radio label="Evenly" value="even" />
                <v-radio label="By amount" value="amount" />
              </v-radio-group>
              <a
                v-if="splitMode === 'even'"
                class="tw-flex-none tw-text-xs tw-text-brass"
                @click="toggleAll"
              >
                {{ allSelected ? "Clear all" : "Select all" }}
              </a>
            </div>

            <div
              v-for="person in participants"
              :key="person.id"
              class="tw-flex tw-items-center tw-gap-2 tw-py-0.5"
            >
              <v-checkbox
                :input-value="isSelected(person.id)"
                :label="person.name"
                dense
                hide-details
                class="tw-mt-0 tw-flex-grow tw-pt-0"
                @change="setSelected(person.id, $event)"
              />

              <!-- Even mode shows the computed share, read-only; by-amount
                   swaps in a field. Both are v-if for the reason above. -->
              <div
                v-if="splitMode === 'even'"
                class="tw-w-20 tw-flex-none tw-text-right tw-text-sm tw-text-parchment-dim"
              >
                {{
                  isSelected(person.id)
                    ? formatCents(evenShare(person.id))
                    : "—"
                }}
              </div>
              <v-text-field
                v-else
                :model-value="typedAmounts[person.id]"
                :disabled="!isSelected(person.id)"
                prefix="$"
                inputmode="decimal"
                dense
                hide-details
                class="tw-w-24 tw-flex-none tw-pt-0 tw-text-sm"
                @input="setTypedAmount(person.id, $event)"
              />
            </div>

            <div
              class="tw-mt-2 tw-flex tw-items-center tw-justify-between tw-border-t tw-border-brass-dim tw-pt-2 tw-text-sm"
            >
              <span class="tw-text-parchment-dim">{{ splitSummary }}</span>
              <span :class="splitTotalClass">{{
                formatCents(splitTotal)
              }}</span>
            </div>
          </template>
        </template>

        <!-- Receipts -->
        <div class="tw-mt-4">
          <div class="tw-mb-1 tw-flex tw-items-center tw-justify-between">
            <span class="tw-text-sm tw-font-medium">Receipts</span>
            <v-btn
              small
              text
              class="tw-text-brass"
              :disabled="!canAddPhoto"
              :loading="processingPhoto"
              @click="pickFile"
            >
              <v-icon small left>mdi-camera-plus-outline</v-icon>
              Add photo
            </v-btn>
          </div>

          <input
            ref="file"
            type="file"
            accept="image/*"
            multiple
            class="tw-hidden"
            @change="onFilesChosen"
          />

          <div v-if="photoError" class="tw-mb-1 tw-text-xs tw-text-red">
            {{ photoError }}
          </div>

          <div
            v-if="pendingPhotos.length"
            class="tw-flex tw-flex-wrap tw-gap-2"
          >
            <div
              v-for="(photo, i) in pendingPhotos"
              :key="i"
              class="tw-relative tw-h-16 tw-w-16 tw-overflow-hidden tw-rounded tw-border tw-border-brass-dim"
            >
              <img
                :src="photo.dataUrl"
                class="tw-h-full tw-w-full tw-object-cover"
              />
              <v-btn
                icon
                x-small
                class="tw-absolute tw-right-0 tw-top-0 tw-bg-wood-deep/80 tw-text-parchment"
                title="Remove"
                @click="pendingPhotos.splice(i, 1)"
              >
                <v-icon x-small>mdi-close</v-icon>
              </v-btn>
            </div>
          </div>
          <div v-else class="tw-text-xs tw-text-parchment-dim">
            {{
              expense
                ? "Photos already attached are managed from the ledger."
                : "None yet."
            }}
          </div>
        </div>
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn text @click="close">Cancel</v-btn>
        <v-btn
          text
          class="tw-text-brass"
          :disabled="!canSave"
          :loading="saving"
          @click="save"
        >
          {{ expense ? "Save" : "Add expense" }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script>
import { mapState } from "vuex"
import {
  splitEvenlyPreview,
  parseAmount,
  formatCents,
  validateSplits,
  todayIso,
  expenseDateMin,
  expenseDateMax,
  isoToMillis,
  millisToIso,
  formatExpenseDate,
  receiptFileError,
  fitWithin,
  RECEIPT_MAX_EDGE,
  RECEIPT_JPEG_QUALITY,
} from "@/components/event/expenseForm"
import { getExpenseParticipants } from "@/utils/services/ExpenseService"

// Mirrors maxExpenseTitleLength / maxExpenseDescriptionLength on the server,
// which truncates rather than refusing — so these caps only ever prevent
// someone typing text that would silently vanish.
const TITLE_MAX = 120
const DESCRIPTION_MAX = 2000

// Mirrors maxReceiptsPerExpense. Counted across what is already attached and
// what is queued, so an edit cannot queue past the cap and fail on upload.
const MAX_RECEIPTS = 5

/**
 * Add / edit one expense (F22).
 *
 * Owns no persistence: it emits a finished payload and lets EventExpenses do
 * the writing, the same division AvatarEditorDialog uses. What it does own is
 * the arithmetic *preview* — which is the whole reason the split rules are in
 * expenseForm.js rather than inline here, since nothing in a .vue is testable
 * in this project.
 *
 * The server recomputes every split it stores, so nothing here is trusted. The
 * preview exists so that what you agree to is what gets written, down to which
 * two of three people absorb the extra cent.
 */
export default {
  name: "ExpenseDialog",

  props: {
    modelValue: { type: Boolean, default: false },
    /** Short id or _id — whichever the route gave; the server resolves both. */
    eventId: { type: String, required: true },
    /** The row being edited, or null to add a new one. */
    expense: { type: Object, default: null },
  },

  emits: ["update:modelValue", "save"],

  data: () => ({
    date: todayIso(),
    dateMenu: false,
    title: "",
    description: "",
    amount: "",
    paidBy: null,
    splitMode: "even",
    selected: [],
    // Keyed by user id, holding what was TYPED rather than cents: re-parsing on
    // every keystroke would fight the caret in a field mid-edit ("1." is not
    // yet a number, but it is a legitimate thing to have typed).
    typedAmounts: {},

    participants: [],
    loadingParticipants: false,

    pendingPhotos: [],
    processingPhoto: false,
    photoError: "",

    saving: false,
    titleMaxLength: TITLE_MAX,
    descriptionMaxLength: DESCRIPTION_MAX,
  }),

  computed: {
    ...mapState(["authUser"]),

    participantOptions() {
      return this.participants
    },

    dateLabel() {
      return formatExpenseDate(isoToMillis(this.date))
    },

    // Bounds for the picker, matching the server's accepted window (J10).
    dateMin() {
      return expenseDateMin()
    },
    dateMax() {
      return expenseDateMax()
    },

    amountCents() {
      return parseAmount(this.amount)
    },

    amountError() {
      // Silent until something has been typed — an empty field on a form you
      // just opened is not a mistake yet.
      if (this.amount.trim() === "") return ""
      return this.amountCents === null ? "Enter an amount like 42.50" : ""
    },

    allSelected() {
      return (
        this.participants.length > 0 &&
        this.selected.length === this.participants.length
      )
    },

    /** The even split as the server would compute it, keyed by user id. */
    evenShares() {
      const map = {}
      for (const split of splitEvenlyPreview(
        this.amountCents ?? 0,
        this.selected
      )) {
        map[split.userId] = split.amountCents
      }
      return map
    },

    /**
     * The resolved splits in whichever mode is active. By-amount entries that
     * are blank or unparseable count as zero here, so the running total below
     * shows the shortfall rather than disappearing.
     */
    splits() {
      if (this.splitMode === "even") {
        return this.selected.map((userId) => ({
          userId,
          amountCents: this.evenShares[userId] ?? 0,
        }))
      }
      return this.selected.map((userId) => ({
        userId,
        amountCents: parseAmount(this.typedAmounts[userId]) ?? 0,
      }))
    },

    splitCheck() {
      return validateSplits(this.amountCents ?? 0, this.splits)
    },

    splitTotal() {
      return this.splitCheck.total
    },

    splitTotalClass() {
      return this.splitCheck.exact ? "tw-text-parchment" : "tw-text-red"
    },

    splitSummary() {
      if (!this.selected.length) return "Nobody selected"
      const { delta, exact } = this.splitCheck
      if (exact) {
        return `${this.selected.length} ${
          this.selected.length === 1 ? "person" : "people"
        }`
      }
      return delta > 0
        ? `${formatCents(delta)} over`
        : `${formatCents(-delta)} left to assign`
    },

    canAddPhoto() {
      const attached = this.expense?.receipts?.length ?? 0
      return attached + this.pendingPhotos.length < MAX_RECEIPTS
    },

    canSave() {
      return (
        !this.saving &&
        this.title.trim() !== "" &&
        this.amountCents !== null &&
        this.amountCents > 0 &&
        this.paidBy !== null &&
        this.selected.length > 0 &&
        // The server refuses a split that does not reconcile; finding that out
        // on submit rather than while typing would be the worse of the two.
        this.splitCheck.exact
      )
    },
  },

  watch: {
    // Opening is the moment to (re)load — the participant pool can change
    // between one expense and the next as people RSVP.
    value(open) {
      if (open) {
        this.reset()
        this.loadParticipants()
      }
    },

    // An even split re-derives whenever the amount or the people change; a
    // by-amount split is left exactly as typed, which is the point of it.
    amount() {
      this.seedTypedAmounts()
    },
    splitMode() {
      this.seedTypedAmounts()
    },
  },

  methods: {
    formatCents,

    /**
     * Reset the form to the row being edited, or to a blank one.
     *
     * Called on open rather than in created(): the dialog is kept mounted
     * between uses, so created() would run once and leave the second expense
     * showing the first one's numbers.
     */
    reset() {
      const expense = this.expense
      this.date = expense ? millisToIso(expense.date) : todayIso()
      this.title = expense?.title ?? ""
      this.description = expense?.description ?? ""
      this.amount = expense ? (expense.amountCents / 100).toFixed(2) : ""
      this.paidBy = expense?.paidBy ?? this.authUser?._id ?? null
      this.splitMode = expense?.splitMode ?? "even"
      this.selected = expense ? (expense.splits ?? []).map((s) => s.userId) : []
      this.typedAmounts = {}
      if (expense) {
        for (const split of expense.splits ?? []) {
          this.typedAmounts[split.userId] = (split.amountCents / 100).toFixed(2)
        }
      }
      this.pendingPhotos = []
      this.photoError = ""
      this.saving = false
    },

    async loadParticipants() {
      this.loadingParticipants = true
      try {
        const people = (await getExpenseParticipants(this.eventId)) ?? []
        this.participants = people.map((person) => ({
          id: person._id,
          name: this.displayNameFor(person),
          user: person,
        }))

        // A new expense starts with everyone ticked, which is the common case;
        // an edit keeps exactly who it already had.
        if (!this.expense) {
          this.selected = this.participants.map((p) => p.id)
        }
        if (!this.paidBy && this.participants.length) {
          this.paidBy = this.participants[0].id
        }
        this.seedTypedAmounts()
      } catch (err) {
        this.participants = []
      } finally {
        this.loadingParticipants = false
      }
    },

    displayNameFor(person) {
      const name = [person.firstName, person.lastName].filter(Boolean).join(" ")
      return person.nickname || name || "Member"
    },

    isSelected(id) {
      return this.selected.includes(id)
    },

    setSelected(id, on) {
      if (on) {
        if (!this.selected.includes(id)) this.selected.push(id)
      } else {
        this.selected = this.selected.filter((selected) => selected !== id)
      }
      this.seedTypedAmounts()
    },

    toggleAll() {
      this.selected = this.allSelected ? [] : this.participants.map((p) => p.id)
      this.seedTypedAmounts()
    },

    evenShare(id) {
      return this.evenShares[id] ?? 0
    },

    /**
     * Pre-fill the by-amount fields with the even split.
     *
     * Starting from the even division is what makes "one person paid a bit
     * more" a two-field edit rather than a from-scratch arithmetic exercise —
     * and it starts reconciled, so the form is never born in an error state.
     *
     * Only ever writes while the even mode is active or the field is empty, so
     * switching to by-amount and typing does not get overwritten by the next
     * keystroke in the total.
     */
    seedTypedAmounts() {
      if (this.splitMode !== "even") return
      const seeded = {}
      for (const id of this.selected) {
        seeded[id] = ((this.evenShares[id] ?? 0) / 100).toFixed(2)
      }
      this.typedAmounts = seeded
    },

    setTypedAmount(id, value) {
      this.typedAmounts = { ...this.typedAmounts, [id]: value }
    },

    pickFile() {
      this.photoError = ""
      this.$refs.file?.click()
    },

    async onFilesChosen(event) {
      const files = [...(event.target.files ?? [])]
      // Clear immediately: choosing the same file twice in a row fires no
      // change event otherwise.
      event.target.value = ""
      if (!files.length) return

      this.processingPhoto = true
      this.photoError = ""
      try {
        for (const file of files) {
          if (!this.canAddPhoto) {
            this.photoError = `Up to ${MAX_RECEIPTS} photos per expense.`
            break
          }
          const problem = receiptFileError(file)
          if (problem) {
            this.photoError = problem
            continue
          }
          this.pendingPhotos.push({ dataUrl: await this.downscale(file) })
        }
      } catch (err) {
        this.photoError = "That photo could not be read. Try another."
      } finally {
        this.processingPhoto = false
      }
    },

    /**
     * Shrink a photo in the browser before it is ever uploaded.
     *
     * The server caps the request body well below a modern phone photo, so this
     * is not an optimisation — without it a real receipt snap is simply
     * refused. Re-drawing onto a canvas also discards EXIF, so the location the
     * photo was taken never leaves the device; the server strips it again on
     * its own re-encode, which is the check that actually enforces it.
     */
    downscale(file) {
      return new Promise((resolve, reject) => {
        const reader = new FileReader()
        reader.onerror = () => reject(new Error("read failed"))
        reader.onload = () => {
          const img = new Image()
          img.onerror = () => reject(new Error("decode failed"))
          img.onload = () => {
            const { width, height } = fitWithin(
              img.naturalWidth,
              img.naturalHeight,
              RECEIPT_MAX_EDGE
            )
            const canvas = document.createElement("canvas")
            canvas.width = width
            canvas.height = height
            const ctx = canvas.getContext("2d")
            // White underneath: a transparent PNG would otherwise come out
            // black once it is encoded as JPEG, which has no alpha channel.
            ctx.fillStyle = "#ffffff"
            ctx.fillRect(0, 0, width, height)
            ctx.drawImage(img, 0, 0, width, height)
            resolve(canvas.toDataURL("image/jpeg", RECEIPT_JPEG_QUALITY))
          }
          img.src = reader.result
        }
        reader.readAsDataURL(file)
      })
    },

    save() {
      if (!this.canSave) return
      this.saving = true

      const payload = {
        date: isoToMillis(this.date),
        title: this.title.trim(),
        description: this.description.trim(),
        amountCents: this.amountCents,
        paidBy: this.paidBy,
        splitMode: this.splitMode,
      }
      if (this.splitMode === "even") {
        payload.participants = [...this.selected]
      } else {
        payload.splits = this.splits
      }

      this.$emit("save", {
        payload,
        photos: this.pendingPhotos.map((photo) => photo.dataUrl),
      })
    },

    /**
     * Called by the parent once the write has landed or failed — the dialog
     * cannot know which, and closing itself on `save` would hide an error the
     * person still needs to see.
     */
    finish(closed) {
      this.saving = false
      if (closed) this.close()
    },

    close() {
      this.$emit("update:modelValue", false)
    },

    onDialogInput(open) {
      this.$emit("update:modelValue", open)
    },
  },
}
</script>
