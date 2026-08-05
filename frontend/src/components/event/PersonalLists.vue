<template>
  <EventLists
    :lists="lists"
    :can-manage="true"
    :viewer-owns-all="true"
    :collaborative="false"
    drag-group="personal-list-items"
    title="My Lists"
    :refreshing="refreshing"
    @create-list="onCreateList"
    @rename-list="onRenameList"
    @delete-list="onDeleteList"
    @add-item="onAddItem"
    @edit-item="onEditItem"
    @delete-item="onDeleteItem"
    @move-item="onMoveItem"
    @toggle-item-checked="onToggleChecked"
  />
</template>

<script>
import { mapActions } from "vuex"
import EventLists from "@/components/event/EventLists.vue"
import {
  getPersonalLists,
  createPersonalList,
  renamePersonalList,
  deletePersonalList,
  addPersonalListItem,
  editPersonalListItem,
  deletePersonalListItem,
  setPersonalListItemChecked,
  movePersonalListItem,
} from "@/utils/services/PersonalService"

/**
 * The "My Lists" tab (F19): a viewer's own lists on one gathering, private to
 * them.
 *
 * Unlike the shared lists — which live on the `event` object Event.vue owns, so
 * Event.vue has to own their writes too — these are a separate document with
 * exactly one consumer. So this component owns its own fetching and persisting
 * rather than emitting nine more handlers up into a view that is already 1,100
 * lines and is otherwise about calendars and availability. The only thing the
 * parent wants back is the count for the tab label, which is one emit.
 */
export default {
  name: "PersonalLists",

  components: { EventLists },

  props: {
    /** Short id or _id — whichever the route gave; the server resolves both. */
    eventId: { type: String, required: true },
    /**
     * Whether this tab is the one on screen. The band keeps every panel mounted
     * (v-show, not v-if), so there is no created() hook that means "the user
     * just opened this" — this prop going true is that moment.
     */
    active: { type: Boolean, default: false },
  },

  emits: ["loaded"],

  data: () => ({
    lists: [],
    refreshing: false,
  }),

  created() {
    // The tab may already be the selected one on arrival.
    if (this.active) this.refresh()
  },

  watch: {
    active(now, before) {
      if (now && !before) this.refresh()
    },
  },

  methods: {
    ...mapActions(["showError"]),

    /**
     * Re-read the lists.
     *
     * REPLACES the array rather than mutating it: EventLists' `lists` watcher
     * fires on identity change, and an in-place splice would silently stop a
     * just-created list from auto-expanding.
     *
     * The busy guard returns before touching `refreshing`, so it never produces
     * a half transition — every mutation below awaits this exactly once, and
     * the resulting true→false edge is what puts the cursor back in the
     * composer for the next entry.
     */
    async refresh() {
      if (this.refreshing) return
      this.refreshing = true
      try {
        this.lists = (await getPersonalLists(this.eventId)) ?? []
        this.$emit("loaded", this.lists.length)
      } catch (err) {
        this.showError("Could not load your lists. Please try again.")
      } finally {
        this.refreshing = false
      }
    },

    /**
     * Every handler is the same shape: persist, then refetch, and say so
     * plainly if it failed. The refetch is what reconciles — nothing here
     * updates the local array optimistically, so what is on screen is always
     * what the server last said.
     */
    async run(action, message) {
      try {
        await action()
      } catch (err) {
        this.showError(message)
      }
      // Refresh even after a failure: it is what restores the truth on screen
      // when a write half-landed, and it supplies the transition the composer's
      // focus restoration waits on.
      await this.refresh()
    },

    onCreateList(payload) {
      return this.run(
        () => createPersonalList(this.eventId, payload),
        "Could not create that list. Please try again."
      )
    },
    onRenameList({ listId, payload }) {
      return this.run(
        () => renamePersonalList(this.eventId, listId, payload),
        "Could not rename that list. Please try again."
      )
    },
    // A bare id, not an object — matching what EventLists emits (askDeleteList).
    onDeleteList(listId) {
      return this.run(
        () => deletePersonalList(this.eventId, listId),
        "Could not delete that list. Please try again."
      )
    },
    onAddItem({ listId, payload }) {
      return this.run(
        () => addPersonalListItem(this.eventId, listId, payload),
        "Could not add that entry. Please try again."
      )
    },
    onEditItem({ listId, itemId, payload }) {
      return this.run(
        () => editPersonalListItem(this.eventId, listId, itemId, payload),
        "Could not save that entry. Please try again."
      )
    },
    onDeleteItem({ listId, itemId }) {
      return this.run(
        () => deletePersonalListItem(this.eventId, listId, itemId),
        "Could not remove that entry. Please try again."
      )
    },
    onMoveItem({ listId, itemId, payload }) {
      return this.run(
        () => movePersonalListItem(this.eventId, listId, itemId, payload),
        "Could not move that entry."
      )
    },
    onToggleChecked({ listId, itemId, checked }) {
      return this.run(
        () => setPersonalListItemChecked(this.eventId, listId, itemId, checked),
        "Could not update that entry. Please try again."
      )
    },
  },
}
</script>
