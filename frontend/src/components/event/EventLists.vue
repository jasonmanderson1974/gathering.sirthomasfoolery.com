<template>
  <div
    v-if="lists.length || canManage"
    class="tw-mt-3 tw-rounded-md tw-border tw-border-brass-dim tw-bg-leather tw-p-3 tw-text-parchment sm:tw-p-4"
  >
    <div class="tw-mb-2 tw-text-base tw-font-medium">Lists</div>

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

        <!-- Entries (only while the list is open) -->
        <div v-if="isExpanded(list._id)" class="tw-mt-2 tw-space-y-1">
          <div
            v-for="item in itemsOf(list)"
            :key="item._id"
            class="tw-rounded tw-px-2 tw-py-1 hover:tw-bg-brass/10"
          >
            <!-- Inline edit of one's own entry -->
            <template v-if="editingItemId === item._id">
              <LocationInput
                v-if="isLocationList(list)"
                v-model="editText"
                dense
                hide-details
                hide-icon
                autofocus
                @enter="submitEdit(list, item)"
              />
              <v-text-field
                v-else
                v-model="editText"
                dense
                hide-details
                autofocus
                :maxlength="maxItemLength"
                @keyup.enter="submitEdit(list, item)"
                @keyup.esc="cancelEdit"
              />
              <div class="tw-mt-2 tw-flex tw-gap-2">
                <v-btn small text @click="cancelEdit">Cancel</v-btn>
                <v-btn
                  small
                  class="tw-bg-brass tw-text-wood-deep"
                  :disabled="!editText.trim()"
                  @click="submitEdit(list, item)"
                  >Save</v-btn
                >
              </div>
            </template>

            <div v-else class="tw-flex tw-items-start tw-gap-2">
              <v-icon small class="tw-mt-0.5 tw-flex-none tw-text-parchment-dim">
                {{ isLocationList(list) ? "mdi-map-marker" : "mdi-circle-small" }}
              </v-icon>
              <div class="tw-min-w-0 tw-flex-grow">
                <a
                  v-if="isLocationList(list)"
                  :href="mapsUrl(item.text)"
                  target="_blank"
                  rel="noopener"
                  class="tw-break-words tw-text-sm tw-text-brass"
                  >{{ item.text }}</a
                >
                <span v-else class="tw-break-words tw-text-sm">
                  {{ item.text }}
                </span>
                <div class="tw-text-xs tw-text-parchment-dim">
                  {{ item.authorName }}
                </div>
              </div>
              <div class="tw-flex tw-flex-none">
                <v-btn
                  v-if="isMine(item)"
                  icon
                  x-small
                  class="tw-text-parchment-dim"
                  title="Edit entry"
                  @click="startEdit(item)"
                >
                  <v-icon small>mdi-pencil</v-icon>
                </v-btn>
                <v-btn
                  v-if="canDelete(item)"
                  icon
                  x-small
                  class="tw-text-parchment-dim"
                  title="Remove entry"
                  @click="
                    $emit('delete-item', { listId: list._id, itemId: item._id })
                  "
                >
                  <v-icon small>mdi-close</v-icon>
                </v-btn>
              </div>
            </div>
          </div>

          <div
            v-if="!itemsOf(list).length"
            class="tw-px-2 tw-text-sm tw-text-parchment-dim"
          >
            Nothing here yet — add the first.
          </div>
        </div>

        <!-- Add an entry: open to everyone -->
        <div
          v-if="isExpanded(list._id)"
          class="tw-mt-2 tw-border-t tw-border-brass-dim/40 tw-pt-2"
        >
          <LocationInput
            v-if="isLocationList(list)"
            :value="newItemText[list._id] || ''"
            dense
            hide-details
            placeholder="Add a place…"
            @input="setNewItemText(list._id, $event)"
            @enter="submitItem(list)"
          />
          <v-text-field
            v-else
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
        </v-radio-group>
        <div class="tw-mt-1 tw-text-xs tw-text-parchment-dim">
          {{
            newKind === "location"
              ? "Entries are looked up as addresses and link to a map."
              : "Entries are plain text."
          }}
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
import LocationInput from "@/components/LocationInput.vue"

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
    LocationInput,
  },

  props: {
    event: { type: Object, required: true },
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
    // Mirrors the server's caps (routes/event_lists.go) so the field stops at
    // the same point the API would truncate.
    maxNameLength: 100,
    maxItemLength: 300,
  }),

  emits: [
    "create-list",
    "rename-list",
    "delete-list",
    "add-item",
    "edit-item",
    "delete-item",
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
      } else {
        this.expandedLists.splice(i, 1)
        // A draft edit belongs to the open list; leaving it armed would reopen
        // the next expansion mid-edit.
        if (this.editingItemId && this.itemBelongsTo(listId, this.editingItemId)) {
          this.cancelEdit()
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
    isLocationList(list) {
      return list.kind === "location"
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

    submitItem(list) {
      const text = (this.newItemText[list._id] || "").trim()
      if (!text) return
      this.$emit("add-item", { listId: list._id, payload: { text } })
      this.$set(this.newItemText, list._id, "")
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
