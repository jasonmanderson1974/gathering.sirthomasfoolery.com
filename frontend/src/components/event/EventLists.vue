<template>
  <!-- No v-if on the panel: as a side panel it was right to stay out of the
       way when there was nothing to show, but this is a tab now, and selecting
       it must never land on an empty band. -->
  <div
    class="tw-mt-3 tw-rounded-md tw-border tw-border-brass-dim tw-bg-leather tw-p-3 tw-text-parchment sm:tw-p-4"
  >
    <div class="tw-mb-2 tw-flex tw-items-center tw-justify-between">
      <div class="tw-text-base tw-font-medium">{{ title }}</div>
      <!-- Only where someone else could have written since the page loaded.
           On a document only you can write, a refresh button is an invitation
           to wonder what you might be missing. -->
      <v-btn
        v-if="collaborative"
        icon
        size="x-small"
        class="tw-text-parchment-dim"
        title="Refresh lists"
        :disabled="refreshing"
        @click="$emit('refresh')"
      >
        <v-icon size="small">mdi-refresh</v-icon>
      </v-btn>
    </div>

    <!-- Existing lists -->
    <div v-if="lists.length" class="tw-space-y-4">
      <div
        v-for="list in lists"
        :key="list._id"
        class="tw-rounded tw-border tw-border-brass-dim/60 tw-p-2 sm:tw-p-3"
      >
        <!-- Header: the collapse toggle, or the rename field -->
        <div class="tw-flex tw-items-start tw-justify-between tw-gap-2">
          <div v-if="renamingId === list._id" class="tw-flex-grow">
            <v-text-field
              v-model="renameText"
              density="compact"
              hide-details
              autofocus
              :maxlength="maxNameLength"
              @keyup.enter="submitRename(list)"
              @keyup.esc="cancelRename"
            />
            <div class="tw-mt-2 tw-flex tw-gap-2">
              <v-btn size="small" variant="text" @click="cancelRename"
                >Cancel</v-btn
              >
              <v-btn
                size="small"
                class="tw-bg-brass tw-text-wood-deep"
                :disabled="!renameText.trim()"
                @click="submitRename(list)"
                >Save</v-btn
              >
            </div>
          </div>
          <template v-else>
            <div
              class="tw-flex tw-min-w-0 tw-flex-grow tw-cursor-pointer tw-items-start tw-gap-2"
              @click="toggleList(list._id)"
            >
              <v-icon size="small" class="tw-mt-0.5 tw-flex-none">
                {{
                  isExpanded(list._id)
                    ? "mdi-chevron-down"
                    : "mdi-chevron-right"
                }}
              </v-icon>
              <div class="tw-min-w-0">
                <div class="tw-break-words tw-font-medium">{{ list.name }}</div>
                <div class="tw-text-xs tw-text-parchment-dim">
                  {{ itemsOf(list).length }}
                  {{ itemsOf(list).length === 1 ? "entry" : "entries" }}
                </div>
              </div>
            </div>
            <!-- The management buttons sit inside the header, so their clicks
                 must not also toggle the list open or shut. -->
            <div
              v-if="canManage"
              class="tw-flex tw-flex-none"
              :class="{ 'tw-gap-2': phone }"
            >
              <v-btn
                icon
                :x-small="!phone"
                :small="phone"
                class="tw-text-parchment-dim"
                title="Rename list"
                @click.stop="startRename(list)"
              >
                <v-icon size="small">mdi-pencil</v-icon>
              </v-btn>
              <v-btn
                icon
                :x-small="!phone"
                :small="phone"
                class="tw-text-red"
                title="Delete list"
                @click.stop="askDeleteList(list)"
              >
                <v-icon size="small">mdi-delete</v-icon>
              </v-btn>
            </div>
          </template>
        </div>

        <!-- Entries (only while the list is open). Rendered from a flat list of
             precomputed rows rather than a recursive component, so the tree
             building itself is a pure function with tests. -->
        <!-- vuedraggable 4 renders rows through an `#item` scoped slot and keys
             them from `item-key`; the v2 form (a v-for in the default slot)
             throws "draggable element must have an item slot" at runtime. A row
             here is `{ item, depth, ... }`, not the entry itself, so `item-key`
             has to be the function form rather than a field name. -->
        <draggable
          v-if="isExpanded(list._id)"
          :list="rowsOf(list)"
          :item-key="(row) => row.item._id"
          :group="dragGroup"
          draggable=".list-row"
          :delay="200"
          :delay-on-touch-only="true"
          :disabled="refreshing"
          :data-list-id="list._id"
          class="tw-mt-2 tw-min-h-[2rem] tw-space-y-1"
          @start="onDragStart"
          @end="onDragEnd"
        >
          <!-- In the header slot, not among the rows: it must not count as a
               drop position, and an empty list still has to be a target. -->
          <template #header>
            <div
              v-if="!itemsOf(list).length"
              class="tw-px-2 tw-text-sm tw-text-parchment-dim"
            >
              Nothing here yet — add the first.
            </div>
          </template>
          <template #item="{ element: row }">
            <div
              :data-item-id="row.item._id"
              :class="`list-row tw-rounded tw-px-2 tw-py-1 hover:tw-bg-brass/10 ${indentClass(
                row.depth
              )}`"
            >
              <!-- Inline edit of one's own entry -->
              <template v-if="editingItemId === row.item._id">
                <LocationInput
                  v-if="isLocationList(list)"
                  v-model="editText"
                  dense
                  hide-details
                  hide-icon
                  autofocus
                  @enter="submitEdit(list, row.item)"
                />
                <v-text-field
                  v-else
                  v-model="editText"
                  density="compact"
                  hide-details
                  autofocus
                  :maxlength="maxItemLength"
                  @keyup.enter="submitEdit(list, row.item)"
                  @keyup.esc="cancelEdit"
                />
                <div class="tw-mt-2 tw-flex tw-gap-2">
                  <v-btn size="small" variant="text" @click="cancelEdit"
                    >Cancel</v-btn
                  >
                  <v-btn
                    size="small"
                    class="tw-bg-brass tw-text-wood-deep"
                    :disabled="!editText.trim()"
                    @click="submitEdit(list, row.item)"
                    >Save</v-btn
                  >
                </div>
              </template>

              <div v-else class="tw-flex tw-items-start tw-gap-2">
                <!-- Collapse toggle, in a fixed-width slot that stays empty on an
                   entry with no children, so text lines up down the column. -->
                <div class="tw-mt-0.5 tw-w-4 tw-flex-none">
                  <v-icon
                    v-if="row.hasChildren"
                    size="small"
                    class="tw-cursor-pointer tw-text-parchment-dim"
                    :title="
                      row.collapsed ? 'Show sub-entries' : 'Hide sub-entries'
                    "
                    @click="toggleItem(row.item._id)"
                  >
                    {{
                      row.collapsed ? "mdi-chevron-right" : "mdi-chevron-down"
                    }}
                  </v-icon>
                </div>

                <!-- The entry's own marker: a checkbox anyone may tick, a map pin,
                   or a plain bullet. -->
                <v-icon
                  v-if="isChecklist(list)"
                  size="small"
                  class="tw-mt-0.5 tw-flex-none tw-cursor-pointer tw-text-brass"
                  :title="row.item.checked ? 'Uncheck' : 'Check off'"
                  @click="toggleChecked(list, row.item)"
                >
                  {{
                    row.item.checked
                      ? "mdi-checkbox-marked"
                      : "mdi-checkbox-blank-outline"
                  }}
                </v-icon>
                <v-icon
                  v-else
                  size="small"
                  class="tw-mt-0.5 tw-flex-none tw-text-parchment-dim"
                >
                  {{
                    isLocationList(list) ? "mdi-map-marker" : "mdi-circle-small"
                  }}
                </v-icon>

                <div class="tw-min-w-0 tw-flex-grow">
                  <a
                    v-if="isLocationList(list)"
                    :href="mapsUrl(row.item.text)"
                    target="_blank"
                    rel="noopener"
                    class="tw-break-words tw-text-sm tw-text-brass"
                    >{{ row.item.text }}</a
                  >
                  <span
                    v-else
                    :class="`tw-break-words tw-text-sm ${
                      row.item.checked
                        ? 'tw-text-parchment-dim tw-line-through'
                        : ''
                    }`"
                  >
                    {{ row.item.text }}
                  </span>
                  <!-- Conditional on the DATA, not on a prop. A private entry
                     carries no authorName and no checker, so an unconditional
                     line would leave an empty row of whitespace under every
                     one of them — and this also tidies a shared entry whose
                     author no longer resolves. -->
                  <div
                    v-if="row.item.authorName || checkLabel(row.item)"
                    class="tw-text-xs tw-text-parchment-dim"
                  >
                    {{ row.item.authorName
                    }}<span v-if="checkLabel(row.item)">
                      · {{ checkLabel(row.item) }}</span
                    >
                  </div>
                </div>
                <div
                  class="tw-flex tw-flex-none"
                  :class="{ 'tw-gap-2': phone }"
                >
                  <v-btn
                    v-if="canNest(row)"
                    icon
                    :x-small="!phone"
                    :small="phone"
                    class="tw-text-parchment-dim"
                    title="Add sub-entry"
                    @click="startChild(row.item)"
                  >
                    <v-icon size="small">mdi-plus</v-icon>
                  </v-btn>
                  <v-btn
                    v-if="isMine(row.item)"
                    icon
                    :x-small="!phone"
                    :small="phone"
                    class="tw-text-parchment-dim"
                    title="Edit entry"
                    @click="startEdit(row.item)"
                  >
                    <v-icon size="small">mdi-pencil</v-icon>
                  </v-btn>
                  <v-btn
                    v-if="canDelete(row.item)"
                    icon
                    :x-small="!phone"
                    :small="phone"
                    class="tw-text-parchment-dim"
                    :title="
                      row.hasChildren
                        ? 'Remove entry and its sub-entries'
                        : 'Remove entry'
                    "
                    @click="askDeleteItem(list, row.item)"
                  >
                    <v-icon size="small">mdi-close</v-icon>
                  </v-btn>
                </div>
              </div>

              <!-- Sub-entry composer, one at a time, directly under its parent -->
              <div
                v-if="addingChildOf === row.item._id"
                class="tw-mt-1 tw-pl-6"
              >
                <LocationInput
                  v-if="isLocationList(list)"
                  ref="childInput"
                  v-model="childText"
                  dense
                  hide-details
                  hide-icon
                  autofocus
                  placeholder="Add a place…"
                  @enter="submitChild(list)"
                />
                <v-text-field
                  v-else
                  ref="childInput"
                  v-model="childText"
                  density="compact"
                  hide-details
                  autofocus
                  placeholder="Add a sub-entry…"
                  :maxlength="maxItemLength"
                  @keyup.enter="submitChild(list)"
                  @keyup.esc="cancelChild"
                />
                <div class="tw-mt-2 tw-flex tw-gap-2">
                  <v-btn size="small" variant="text" @click="cancelChild"
                    >Cancel</v-btn
                  >
                  <v-btn
                    size="small"
                    class="tw-bg-brass tw-text-wood-deep"
                    :disabled="!childText.trim()"
                    @click="submitChild(list)"
                    >Add</v-btn
                  >
                </div>
              </div>
            </div>
          </template>
        </draggable>

        <!-- Add an entry: open to everyone -->
        <div
          v-if="isExpanded(list._id)"
          class="tw-mt-2 tw-border-t tw-border-brass-dim/40 tw-pt-2"
        >
          <LocationInput
            v-if="isLocationList(list)"
            :ref="`addInput-${list._id}`"
            :model-value="newItemText[list._id] || ''"
            dense
            hide-details
            placeholder="Add a place…"
            @update:model-value="setNewItemText(list._id, $event)"
            @enter="submitItem(list)"
          />
          <v-text-field
            v-else
            :ref="`addInput-${list._id}`"
            :model-value="newItemText[list._id] || ''"
            density="compact"
            hide-details
            placeholder="Add an entry…"
            :maxlength="maxItemLength"
            @update:model-value="setNewItemText(list._id, $event)"
            @keyup.enter="submitItem(list)"
          />
          <div class="tw-mt-2 tw-flex tw-justify-end">
            <v-btn
              size="small"
              variant="outlined"
              class="tw-text-brass"
              :disabled="!(newItemText[list._id] || '').trim()"
              @click="submitItem(list)"
            >
              <v-icon size="small" left>mdi-plus</v-icon>
              Add
            </v-btn>
          </div>
        </div>
      </div>
    </div>
    <div v-else-if="!showNewList" class="tw-text-sm tw-text-parchment-dim">
      No lists yet.
    </div>

    <!-- Planner: create a new list -->
    <template v-if="canManage">
      <div
        v-if="showNewList"
        class="tw-mt-3 tw-border-t tw-border-brass-dim tw-pt-3"
      >
        <v-text-field
          v-model="newName"
          label="List name (e.g. Menu)"
          density="compact"
          hide-details
          :maxlength="maxNameLength"
          class="tw-mb-2"
          @keyup.enter="createList"
        />
        <v-radio-group v-model="newKind" row dense hide-details class="tw-mt-1">
          <v-radio label="Anything" value="text" />
          <v-radio label="Places" value="location" />
          <v-radio label="Checklist" value="checklist" />
        </v-radio-group>
        <div class="tw-mt-1 tw-text-xs tw-text-parchment-dim">
          {{ kindHint }}
        </div>
        <div class="tw-mt-3 tw-flex tw-gap-2">
          <v-btn size="small" variant="text" @click="cancelNewList"
            >Cancel</v-btn
          >
          <v-btn
            size="small"
            class="tw-bg-brass tw-text-wood-deep"
            :disabled="!newName.trim()"
            @click="createList"
            >Create list</v-btn
          >
        </div>
      </div>
      <v-btn
        v-else
        size="small"
        variant="outlined"
        class="tw-mt-3 tw-text-brass"
        @click="showNewList = true"
      >
        <v-icon size="small" left>mdi-plus</v-icon>
        Add list
      </v-btn>
    </template>

    <ConfirmDeleteDialog
      :model-value="!!pendingDelete"
      :title="pendingDelete ? pendingDelete.title : ''"
      :body="pendingDelete ? pendingDelete.body : ''"
      @update:model-value="onConfirmDialogInput"
      @confirm="confirmDelete"
    />
  </div>
</template>

<script>
import { mapGetters, mapState } from "vuex"
import { mapsSearchUrl, isPhone } from "@/utils"
import draggable from "vuedraggable"
import LocationInput from "@/components/LocationInput.vue"
import ConfirmDeleteDialog from "@/components/general/ConfirmDeleteDialog.vue"
import {
  flattenListItems,
  canAddChild,
  checkStateLabel,
  orderBetween,
  resolveDrop,
  describeListDeletion,
  describeItemDeletion,
} from "@/components/event/eventLists"

/**
 * A panel of lists — "Menu", "Bars to Visit" (F14) — rendering, drag, nesting
 * and the delete dialogs. Presentational: it reads the `lists` prop and emits
 * for its parent to persist and refetch, the same contract EventPolls uses.
 *
 * ONE component serves two panels (F19). The shared lists on an event use the
 * defaults below; a viewer's own private lists pass `viewerOwnsAll`,
 * `collaborative: false` and their own `dragGroup`. It is deliberately not two
 * components: the drag resolution, the three-level tree and the focus handling
 * were each got wrong once already, and a copy would have to be got right
 * twice every time.
 *
 * The rights the shared panel shows mirror the server (see
 * routes/event_lists.go): the planner or an admin creates, renames and deletes
 * the lists; anyone signed in adds entries; an entry may be edited only by its
 * author, but removed by its author or by any member. These checks are for the
 * UI's benefit only — the server enforces the same rules and is what actually
 * decides.
 */
export default {
  name: "EventLists",

  components: {
    ConfirmDeleteDialog,
    draggable,
    LocationInput,
  },

  props: {
    /**
     * The lists to render, passed in rather than read off an event.
     *
     * The parent must REPLACE this array on refresh, never mutate it in place:
     * the `lists` watcher below fires on identity change, and an in-place
     * splice would silently stop a just-created list from auto-expanding.
     */
    lists: { type: Array, required: true },
    /**
     * Whether this viewer may create, rename and delete whole lists. Hoisted to
     * the parent so the rule lives in `canManageEventLists` (eventLists.js),
     * where it can be tested — it could not be, inside a .vue.
     */
    canManage: { type: Boolean, default: false },
    /**
     * True when every entry here belongs to the viewer, which collapses the
     * per-entry edit and delete rights to always-allowed. Not cosmetic: it is
     * what makes a private panel private-feeling rather than one where you
     * appear to need permission from yourself.
     */
    viewerOwnsAll: { type: Boolean, default: false },
    /**
     * Whether anyone else can be writing to these lists. Governs BOTH the
     * refresh button and the refetch-on-expand — they exist for the same
     * reason, and neither means anything on a document only you can write.
     */
    collaborative: { type: Boolean, default: true },
    /**
     * The vuedraggable group name. MUST differ between panels: both are mounted
     * at once (the band uses v-show, not v-if), so a shared name would let an
     * entry be dragged out of the club's lists into a private one — and the
     * move it emitted would name a list id from the other collection entirely.
     */
    dragGroup: { type: String, default: "event-list-items" },
    /** The panel heading: "Lists", or "My Lists". */
    title: { type: String, default: "Lists" },
    /**
     * True while the parent is refetching, so the refresh button can't be
     * hammered into a queue of overlapping requests. Load-bearing beyond that:
     * the true→false TRANSITION is what restores the cursor after an add (see
     * applyPendingFocus), so a parent must produce exactly one per mutation.
     */
    refreshing: { type: Boolean, default: false },
  },

  data: () => ({
    showNewList: false,
    newName: "",
    newKind: "text",
    // Draft entry text per list id, so typing in one list doesn't disturb another.
    newItemText: {},
    // Ids of the lists currently open. Lists start collapsed, like the
    // discussion's threads: the panel is a scannable index of what needs
    // filling in, and you open the one you came for.
    expandedLists: [],
    // Name of a list this viewer just created, so it can be opened as soon as
    // the refetch brings it back with an id.
    pendingExpandName: null,
    renamingId: null,
    renameText: "",
    editingItemId: null,
    editText: "",
    // Ids of entries whose sub-entries are hidden. Sub-entries start SHOWN —
    // the list header is already the collapse unit, and a subtree is small.
    collapsedItemIds: [],
    // The one entry currently taking a sub-entry, and its draft. One at a time,
    // like editingItemId: a keyed map would repeat the reactivity dance the
    // per-list drafts need, for no gain when only one composer is ever open.
    addingChildOf: null,
    childText: "",
    // Where to put the cursor once the refetch that follows an add has landed:
    // `{ type: "add", listId }` or `{ type: "child" }`, or null for nowhere.
    // Adding is the one action people do several times in a row — "Beer ⏎ chips
    // ⏎ ice ⏎" — and every add round-trips through the server and re-renders
    // from new data, which is what would otherwise drop the cursor.
    pendingFocus: null,
    // The entry being dragged, if any. Its subtree is hidden for the duration
    // so the whole thing travels as one row — see rowsOf.
    draggingItemId: null,
    // The deletion waiting to be confirmed: the dialog's wording plus the emit
    // to make if it is. Null when the dialog is shut. Holding the emit itself
    // means the dialog stays ignorant of what it is deleting, and both kinds of
    // deletion — a whole list, one entry — go through the same path.
    pendingDelete: null,
    // Mirrors the server's caps (routes/event_lists.go) so the field stops at
    // the same point the API would truncate.
    maxNameLength: 100,
    maxItemLength: 300,
  }),

  emits: [
    "refresh",
    "create-list",
    "rename-list",
    "delete-list",
    "add-item",
    "edit-item",
    "delete-item",
    "move-item",
    "toggle-item-checked",
  ],

  computed: {
    ...mapState(["authUser"]),
    ...mapGetters(["canInvite"]),
    // The icon buttons on a row sit close together and are the controls most
    // likely to be used one-handed, standing in a shop. $vuetify.breakpoint is
    // reactive, so this follows a rotation without a resize listener.
    phone() {
      return isPhone(this.$vuetify)
    },
    kindHint() {
      if (this.newKind === "location") {
        return "Entries are looked up as addresses and link to a map."
      }
      if (this.newKind === "checklist") {
        return "Entries get a checkbox anyone can tick off."
      }
      return "Entries are plain text."
    },
  },

  watch: {
    // Event.vue persists then refetches, so a list this viewer just created
    // arrives here as new data. Open it: whoever named a list means to start
    // filling it in, and its add-entry field is inside the collapsed body.
    lists(current) {
      if (!this.pendingExpandName) return
      // Searched from the end: a new list is appended, so with two of the same
      // name this opens the one just made rather than the older one.
      const created = [...current]
        .reverse()
        .find((l) => l.name === this.pendingExpandName)
      if (!created) return
      this.pendingExpandName = null
      if (!this.expandedLists.includes(created._id)) {
        this.expandedLists.push(created._id)
      }
    },

    // An add is emitted, persisted and refetched, so the composer only settles
    // once the parent stops refreshing. Watching the TRANSITION back to false is
    // what makes this land after the new rows have rendered rather than during.
    refreshing(now, before) {
      if (before && !now) this.applyPendingFocus()
    },
  },

  methods: {
    mapsUrl(text) {
      return mapsSearchUrl(text)
    },
    isExpanded(listId) {
      return this.expandedLists.includes(listId)
    },
    toggleList(listId) {
      const i = this.expandedLists.indexOf(listId)
      if (i === -1) {
        this.expandedLists.push(listId)
        // Opening a list is a request to read it, so fetch it fresh — this is
        // the moment someone else's entries are most likely to be missing.
        // Only where there IS someone else: on a private list the fetch could
        // only ever return what is already on screen.
        if (this.collaborative) this.$emit("refresh")
      } else {
        this.expandedLists.splice(i, 1)
        // A draft edit or sub-entry belongs to the open list; leaving one armed
        // would reopen the next expansion mid-compose.
        if (
          this.editingItemId &&
          this.itemBelongsTo(listId, this.editingItemId)
        ) {
          this.cancelEdit()
        }
        if (
          this.addingChildOf &&
          this.itemBelongsTo(listId, this.addingChildOf)
        ) {
          this.cancelChild()
        }
      }
    },
    itemBelongsTo(listId, itemId) {
      const list = this.lists.find((l) => l._id === listId)
      return this.itemsOf(list ?? {}).some((i) => i._id === itemId)
    },
    itemsOf(list) {
      return list.items ?? []
    },
    // The tree, flattened to rows with a depth. Built in a pure module so the
    // nesting rules are testable — nothing inside a .vue file is.
    rowsOf(list) {
      return flattenListItems(this.itemsOf(list), this.collapsedIdsForRender())
    },
    // The entry being dragged is collapsed on top of whatever the viewer has
    // collapsed themselves, and only for the duration of the drag: its subtree
    // follows the ghost as one row instead of being left behind, and the drop
    // indices stay in step with what is on screen. The viewer's own collapse
    // state is never written to.
    collapsedIdsForRender() {
      if (!this.draggingItemId) return this.collapsedItemIds
      return [...this.collapsedItemIds, this.draggingItemId]
    },
    // Static classes, not a computed `tw-pl-${n}`: Tailwind purges on literal
    // source text, so a built-up class name emits no CSS at all.
    indentClass(depth) {
      return ["", "tw-pl-6", "tw-pl-12"][depth] ?? "tw-pl-12"
    },
    toggleItem(itemId) {
      const i = this.collapsedItemIds.indexOf(itemId)
      if (i === -1) {
        this.collapsedItemIds.push(itemId)
      } else {
        this.collapsedItemIds.splice(i, 1)
      }
    },
    // A grandchild takes no children, so it isn't offered the control.
    canNest(row) {
      return !!this.authUser && canAddChild(row.depth)
    },
    checkLabel(item) {
      return checkStateLabel(item)
    },
    isLocationList(list) {
      return list.kind === "location"
    },
    isChecklist(list) {
      return list.kind === "checklist"
    },
    // Open to anyone signed in, guests included — the server agrees.
    toggleChecked(list, item) {
      if (!this.authUser) return
      this.$emit("toggle-item-checked", {
        listId: list._id,
        itemId: item._id,
        checked: !item.checked,
      })
    },
    isMine(item) {
      // On a private panel there is nobody else's entry to be looking at, and
      // the id comparison is not merely redundant there — it is the wrong
      // question to be asking about your own notebook.
      if (this.viewerOwnsAll) return true
      return !!this.authUser && item.userId === this.authUser._id
    },
    // Own entries always; anyone's from member upwards.
    canDelete(item) {
      if (this.viewerOwnsAll) return true
      return this.isMine(item) || this.canInvite
    },

    // v-text-field can't v-model straight into a keyed object, so the drafts
    // go through here and the object is replaced rather than mutated.
    setNewItemText(listId, value) {
      this.newItemText = { ...this.newItemText, [listId]: value }
    },

    /**
     * Put the cursor back where the last add came from, once the row it created
     * has arrived.
     *
     * A v-for collects refs as an array even where only one element can match,
     * hence the unwrap. Every step is optional-chained: the composer's list may
     * have been collapsed, or the whole list deleted by someone else, while the
     * add was in flight — in which case there is simply nothing to focus, and
     * doing nothing is the right answer.
     */
    applyPendingFocus() {
      const pending = this.pendingFocus
      if (!pending) return
      this.pendingFocus = null
      this.$nextTick(() => {
        const ref =
          pending.type === "child"
            ? this.$refs.childInput
            : this.$refs[`addInput-${pending.listId}`]
        const target = Array.isArray(ref) ? ref[0] : ref
        target?.focus?.()
      })
    },

    submitItem(list) {
      const text = (this.newItemText[list._id] || "").trim()
      if (!text) return
      this.$emit("add-item", { listId: list._id, payload: { text } })
      this.newItemText = { ...this.newItemText, [list._id]: "" }
      this.pendingFocus = { type: "add", listId: list._id }
    },

    startChild(item) {
      this.addingChildOf = item._id
      this.childText = ""
      // A hidden subtree would swallow what you're about to add.
      const i = this.collapsedItemIds.indexOf(item._id)
      if (i !== -1) this.collapsedItemIds.splice(i, 1)
    },
    cancelChild() {
      this.addingChildOf = null
      this.childText = ""
      // Closing the composer withdraws its claim on the cursor, but not one the
      // main add field may have staked in the meantime.
      if (this.pendingFocus?.type === "child") this.pendingFocus = null
    },
    // Deliberately does NOT cancelChild(): the composer stays open on the same
    // parent so a run of sub-entries can be typed without reaching for the
    // "add sub-entry" button between each one. Esc and Cancel are what close
    // it, and collapsing the list still closes it via toggleList.
    submitChild(list) {
      const text = this.childText.trim()
      if (!text) return
      this.$emit("add-item", {
        listId: list._id,
        payload: { text, parentId: this.addingChildOf },
      })
      this.childText = ""
      this.pendingFocus = { type: "child" }
    },

    // Deleting a list takes every entry on it; deleting an entry takes its
    // sub-entries, other people's included. Neither used to ask.
    askDeleteList(list) {
      const { title, body } = describeListDeletion(list)
      this.pendingDelete = {
        title,
        body,
        event: "delete-list",
        payload: list._id,
      }
    },
    askDeleteItem(list, item) {
      const { title, body } = describeItemDeletion(this.itemsOf(list), item._id)
      this.pendingDelete = {
        title,
        body,
        event: "delete-item",
        payload: { listId: list._id, itemId: item._id },
      }
    },
    // The dialog reports its own dismissal — Esc, or the backdrop — the same
    // way it reports Cancel.
    onConfirmDialogInput(open) {
      if (!open) this.pendingDelete = null
    },
    confirmDelete() {
      const pending = this.pendingDelete
      this.pendingDelete = null
      if (!pending) return
      this.$emit(pending.event, pending.payload)
    },

    onDragStart(evt) {
      this.draggingItemId = evt?.item?.dataset?.itemId ?? null
    },

    /**
     * Turn a drop into a move (F18).
     *
     * vuedraggable has already moved the row in the DOM and spliced the
     * throwaway rows array it was handed; neither is the truth. The truth is
     * what comes back from the refetch, so this only has to work out where the
     * row was released and say so.
     *
     * The rows are rebuilt here rather than read off the last render, with the
     * dragged entry still collapsed, so the indices Sortable reported are
     * measured against exactly the rows they were computed from.
     */
    onDragEnd(evt) {
      const collapsed = this.collapsedIdsForRender()
      this.draggingItemId = null

      const sourceListId = evt?.from?.dataset?.listId
      const targetListId = evt?.to?.dataset?.listId
      const itemId = evt?.item?.dataset?.itemId
      if (!sourceListId || !targetListId || !itemId) return

      // A refresh can land mid-drag and take a list with it. A drop with
      // nowhere to come from or go to is simply dropped: the refetch already
      // showed what is really there.
      const sourceList = this.lists.find((l) => l._id === sourceListId)
      const targetList = this.lists.find((l) => l._id === targetListId)
      if (!sourceList || !targetList) return

      const draggedItem = this.itemsOf(sourceList).find((i) => i._id === itemId)
      if (!draggedItem) return

      const sameList = sourceListId === targetListId
      const sourceRows = flattenListItems(this.itemsOf(sourceList), collapsed)
      const targetRows = sameList
        ? sourceRows
        : flattenListItems(this.itemsOf(targetList), collapsed)

      // The *Draggable* indices, not evt.oldIndex/newIndex: those count every
      // element in the container, and the empty-state header is one of them.
      // These count only what matches the `draggable` selector, which is
      // exactly the rows resolveDrop is given.
      const { parentId, prevOrder, nextOrder } = resolveDrop({
        sourceRows,
        targetRows,
        oldIndex: evt.oldDraggableIndex ?? evt.oldIndex,
        newIndex: evt.newDraggableIndex ?? evt.newIndex,
        sameList,
        draggedItem,
      })

      const payload = {
        targetListId,
        order: orderBetween(prevOrder, nextOrder),
      }
      // Only sent for a reorder among siblings the entry already has; the
      // server refuses any other parentId, which is what stops a drag from
      // nesting an entry deeper.
      if (parentId) payload.parentId = parentId

      this.$emit("move-item", { listId: sourceListId, itemId, payload })
    },

    startEdit(item) {
      this.editingItemId = item._id
      this.editText = item.text
    },
    cancelEdit() {
      this.editingItemId = null
      this.editText = ""
    },
    submitEdit(list, item) {
      const text = this.editText.trim()
      if (!text) return
      this.$emit("edit-item", {
        listId: list._id,
        itemId: item._id,
        payload: { text },
      })
      this.cancelEdit()
    },

    startRename(list) {
      this.renamingId = list._id
      this.renameText = list.name
    },
    cancelRename() {
      this.renamingId = null
      this.renameText = ""
    },
    submitRename(list) {
      const name = this.renameText.trim()
      if (!name) return
      this.$emit("rename-list", { listId: list._id, payload: { name } })
      this.cancelRename()
    },

    createList() {
      const name = this.newName.trim()
      if (!name) return
      this.$emit("create-list", { name, kind: this.newKind })
      this.pendingExpandName = name
      this.cancelNewList()
    },
    cancelNewList() {
      this.showNewList = false
      this.newName = ""
      this.newKind = "text"
    },
  },
}
</script>
