<template>
  <!-- No v-if on the panel: as a side panel it was right to stay out of the
       way when there was nothing to show, but this is a tab now, and selecting
       it must never land on an empty band. -->
  <div
    class="tw-mt-3 tw-rounded-md tw-border tw-border-brass-dim tw-bg-leather tw-p-3 tw-text-parchment sm:tw-p-4"
  >
    <div class="tw-mb-2 tw-flex tw-items-center tw-justify-between">
      <div class="tw-text-base tw-font-medium">Lists</div>
      <v-btn
        icon
        x-small
        class="tw-text-parchment-dim"
        title="Refresh lists"
        :disabled="refreshing"
        @click="$emit('refresh')"
      >
        <v-icon small>mdi-refresh</v-icon>
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
              dense
              hide-details
              autofocus
              :maxlength="maxNameLength"
              @keyup.enter="submitRename(list)"
              @keyup.esc="cancelRename"
            />
            <div class="tw-mt-2 tw-flex tw-gap-2">
              <v-btn small text @click="cancelRename">Cancel</v-btn>
              <v-btn
                small
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
              <v-icon small class="tw-mt-0.5 tw-flex-none">
                {{ isExpanded(list._id) ? "mdi-chevron-down" : "mdi-chevron-right" }}
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
            <div v-if="canManage" class="tw-flex tw-flex-none">
              <v-btn
                icon
                x-small
                class="tw-text-parchment-dim"
                title="Rename list"
                @click.stop="startRename(list)"
              >
                <v-icon small>mdi-pencil</v-icon>
              </v-btn>
              <v-btn
                icon
                x-small
                class="tw-text-red"
                title="Delete list"
                @click.stop="$emit('delete-list', list._id)"
              >
                <v-icon small>mdi-delete</v-icon>
              </v-btn>
            </div>
          </template>
        </div>

        <!-- Entries (only while the list is open). Rendered from a flat list of
             precomputed rows rather than a recursive component, so the tree
             building itself is a pure function with tests. -->
        <draggable
          v-if="isExpanded(list._id)"
          :list="rowsOf(list)"
          group="event-list-items"
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
          <div
            v-for="row in rowsOf(list)"
            :key="row.item._id"
            :data-item-id="row.item._id"
            :class="`list-row tw-rounded tw-px-2 tw-py-1 hover:tw-bg-brass/10 ${
              indentClass(row.depth)
            }`"
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
                dense
                hide-details
                autofocus
                :maxlength="maxItemLength"
                @keyup.enter="submitEdit(list, row.item)"
                @keyup.esc="cancelEdit"
              />
              <div class="tw-mt-2 tw-flex tw-gap-2">
                <v-btn small text @click="cancelEdit">Cancel</v-btn>
                <v-btn
                  small
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
                  small
                  class="tw-cursor-pointer tw-text-parchment-dim"
                  :title="row.collapsed ? 'Show sub-entries' : 'Hide sub-entries'"
                  @click="toggleItem(row.item._id)"
                >
                  {{ row.collapsed ? "mdi-chevron-right" : "mdi-chevron-down" }}
                </v-icon>
              </div>

              <!-- The entry's own marker: a checkbox anyone may tick, a map pin,
                   or a plain bullet. -->
              <v-icon
                v-if="isChecklist(list)"
                small
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
                small
                class="tw-mt-0.5 tw-flex-none tw-text-parchment-dim"
              >
                {{ isLocationList(list) ? "mdi-map-marker" : "mdi-circle-small" }}
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
                    row.item.checked ? 'tw-text-parchment-dim tw-line-through' : ''
                  }`"
                >
                  {{ row.item.text }}
                </span>
                <div class="tw-text-xs tw-text-parchment-dim">
                  {{ row.item.authorName
                  }}<span v-if="checkLabel(row.item)">
                    · {{ checkLabel(row.item) }}</span
                  >
                </div>
              </div>
              <div class="tw-flex tw-flex-none">
                <v-btn
                  v-if="canNest(row)"
                  icon
                  x-small
                  class="tw-text-parchment-dim"
                  title="Add sub-entry"
                  @click="startChild(row.item)"
                >
                  <v-icon small>mdi-plus</v-icon>
                </v-btn>
                <v-btn
                  v-if="isMine(row.item)"
                  icon
                  x-small
                  class="tw-text-parchment-dim"
                  title="Edit entry"
                  @click="startEdit(row.item)"
                >
                  <v-icon small>mdi-pencil</v-icon>
                </v-btn>
                <v-btn
                  v-if="canDelete(row.item)"
                  icon
                  x-small
                  class="tw-text-parchment-dim"
                  :title="
                    row.hasChildren
                      ? 'Remove entry and its sub-entries'
                      : 'Remove entry'
                  "
                  @click="
                    $emit('delete-item', {
                      listId: list._id,
                      itemId: row.item._id,
                    })
                  "
                >
                  <v-icon small>mdi-close</v-icon>
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
                dense
                hide-details
                autofocus
                placeholder="Add a sub-entry…"
                :maxlength="maxItemLength"
                @keyup.enter="submitChild(list)"
                @keyup.esc="cancelChild"
              />
              <div class="tw-mt-2 tw-flex tw-gap-2">
                <v-btn small text @click="cancelChild">Cancel</v-btn>
                <v-btn
                  small
                  class="tw-bg-brass tw-text-wood-deep"
                  :disabled="!childText.trim()"
                  @click="submitChild(list)"
                  >Add</v-btn
                >
              </div>
            </div>
          </div>
        </draggable>

        <!-- Add an entry: open to everyone -->
        <div
          v-if="isExpanded(list._id)"
          class="tw-mt-2 tw-border-t tw-border-brass-dim/40 tw-pt-2"
        >
          <LocationInput
            v-if="isLocationList(list)"
            :ref="`addInput-${list._id}`"
            :value="newItemText[list._id] || ''"
            dense
            hide-details
            placeholder="Add a place…"
            @input="setNewItemText(list._id, $event)"
            @enter="submitItem(list)"
          />
          <v-text-field
            v-else
            :ref="`addInput-${list._id}`"
            :value="newItemText[list._id] || ''"
            dense
            hide-details
            placeholder="Add an entry…"
            :maxlength="maxItemLength"
            @input="setNewItemText(list._id, $event)"
            @keyup.enter="submitItem(list)"
          />
          <div class="tw-mt-2 tw-flex tw-justify-end">
            <v-btn
              small
              outlined
              class="tw-text-brass"
              :disabled="!(newItemText[list._id] || '').trim()"
              @click="submitItem(list)"
            >
              <v-icon small left>mdi-plus</v-icon>
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
          dense
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
          <v-btn small text @click="cancelNewList">Cancel</v-btn>
          <v-btn
            small
            class="tw-bg-brass tw-text-wood-deep"
            :disabled="!newName.trim()"
            @click="createList"
            >Create list</v-btn
          >
        </div>
      </div>
      <v-btn
        v-else
        small
        outlined
        class="tw-mt-3 tw-text-brass"
        @click="showNewList = true"
      >
        <v-icon small left>mdi-plus</v-icon>
        Add list
      </v-btn>
    </template>
  </div>
</template>

<script>
import { mapGetters, mapState } from "vuex"
import { mapsSearchUrl } from "@/utils"
import draggable from "vuedraggable"
import LocationInput from "@/components/LocationInput.vue"
import {
  flattenListItems,
  canAddChild,
  checkStateLabel,
  orderBetween,
  resolveDrop,
} from "@/components/event/eventLists"

/**
 * Shared lists on an event (F14) — "Menu", "Bars to Visit". Presentational:
 * reads event.lists and emits for Event.vue to persist + refresh, the same
 * contract EventPolls uses.
 *
 * The rights split mirrors the server (see routes/event_lists.go): the planner
 * or an admin creates, renames and deletes the lists; anyone signed in adds
 * entries; an entry may be edited only by its author, but removed by its author
 * or by any member. These checks are for the UI's benefit only — the server
 * enforces the same rules and is what actually decides.
 */
export default {
  name: "EventLists",

  components: {
    draggable,
    LocationInput,
  },

  props: {
    event: { type: Object, required: true },
    // True while the parent is refetching, so the refresh button can't be
    // hammered into a queue of overlapping requests.
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
    ...mapGetters(["canInvite", "canManageUsers"]),
    lists() {
      return this.event.lists ?? []
    },
    isEventOwner() {
      return (
        !!this.authUser &&
        !!this.event.ownerId &&
        this.event.ownerId !== 0 &&
        this.authUser._id === this.event.ownerId
      )
    },
    // Who may create/rename/delete a whole list. Ownerless legacy events fall
    // back to member+, matching canManageLists on the server.
    canManage() {
      if (!this.authUser) return false
      if (this.canManageUsers) return true
      if (this.event.ownerId && this.event.ownerId !== 0) {
        return this.isEventOwner
      }
      return this.canInvite
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
        this.$emit("refresh")
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
      return !!this.authUser && item.userId === this.authUser._id
    },
    // Own entries always; anyone's from member upwards.
    canDelete(item) {
      return this.isMine(item) || this.canInvite
    },

    // v-text-field can't v-model into a keyed object and stay reactive in Vue 2,
    // so drafts are written through $set.
    setNewItemText(listId, value) {
      this.$set(this.newItemText, listId, value)
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
      this.$set(this.newItemText, list._id, "")
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

      const draggedItem = this.itemsOf(sourceList).find(
        (i) => i._id === itemId
      )
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
