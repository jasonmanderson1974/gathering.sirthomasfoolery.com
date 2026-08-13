import { createStore } from "vuex"
import { get } from "@/utils"
import { roleGetters } from "./role_getters"
import {
  peopleActions,
  peopleGetters,
  peopleMutations,
  peopleState,
} from "./people"
import {
  createFolder,
  deleteFolder,
  setEventFolder,
  updateFolder,
} from "../utils/services/FolderService"
import { archiveEvent } from "../utils/services/EventService"
import { setCacheUser } from "../utils/offline/cache"
import { rememberSession } from "../utils/offline/session"
import { setOfflineViewer } from "../utils/offline/writes"
import { prefetchGatherings } from "../utils/offline/prefetch"

export default createStore({
  state: {
    error: "",
    info: "",

    authUser: null,

    // Whether the server is currently unreachable. Driven by real request
    // outcomes rather than navigator.onLine — see utils/offline/status.js.
    offline: false,

    events: [],
    folders: [],

    // The Fellowship, cached by account id, for the hover card (N3).
    ...peopleState(),

    // Feature flags
    daysOnlyEnabled: true,
    overlayAvailabilitiesEnabled: true,

    // New dialog
    newDialogOptions: {
      show: false,
      contactsPayload: {},
      folderId: null,
    },
  },
  getters: {
    ...roleGetters,
    ...peopleGetters,
  },
  mutations: {
    setError(state, error) {
      state.error = error
    },
    setInfo(state, info) {
      state.info = info
    },

    setAuthUser(state, authUser) {
      state.authUser = authUser

      // The one place the app learns who is signed in, so it is where the
      // offline layer is told too. Both calls matter on the NEXT cold boot:
      // the remembered id is how the router knows to offer cached data before
      // the server can be asked, and it is the key the cache is stored under.
      //
      // Only the positive case is handled here, deliberately. `setAuthUser(null)`
      // is committed from three failure catches (App.vue, Home.vue, Event.vue)
      // that fire on a transient error or an offline boot; tearing the cache
      // down there would destroy the data the member is about to read. Teardown
      // belongs where the session is KNOWN to be over — see endOfflineSession.
      // Told either way: the write queue's reducers render the author of a
      // comment made offline, and a stale name there would outlive the session
      // that produced it.
      setOfflineViewer(authUser)
      if (authUser?._id) {
        rememberSession(authUser._id)
        setCacheUser(authUser._id)
      }
    },

    setOffline(state, offline) {
      state.offline = offline
    },

    setEvents(state, events) {
      state.events = events
    },
    setFolders(state, folders) {
      state.folders = folders
    },

    setDaysOnlyEnabled(state, enabled) {
      state.daysOnlyEnabled = enabled
    },
    setOverlayAvailabilitiesEnabled(state, enabled) {
      state.overlayAvailabilitiesEnabled = enabled
    },
    addFolder(state, folder) {
      state.folders.push(folder)
    },
    updateFolder(state, { folderId, name, color }) {
      const folder = state.folders.find((f) => f._id === folderId)
      if (folder) {
        folder.name = name
        folder.color = color
      }
    },
    removeFolder(state, folderId) {
      state.folders = state.folders.filter((f) => f._id !== folderId)
    },
    removeEventFromFolder(state, eventId) {
      state.folders.forEach((folder) => {
        folder.eventIds = folder.eventIds.filter((id) => id !== eventId)
      })
    },
    addEventToFolder(state, { eventId, folderId }) {
      const folder = state.folders.find((f) => f._id === folderId)
      if (folder) {
        folder.eventIds.push(eventId)
      }
    },

    ...peopleMutations,

    setNewDialogOptions(
      state,
      { show = false, contactsPayload = {}, folderId = null }
    ) {
      state.newDialogOptions = {
        show,
        contactsPayload,
        folderId,
      }
    },
  },
  actions: {
    // Error & info
    showError({ commit }, error) {
      commit("setError", "")
      setTimeout(() => commit("setError", error), 0)
    },
    showInfo({ commit }, info) {
      commit("setInfo", "")
      setTimeout(() => commit("setInfo", info), 0)
    },

    // Returns the user as well as committing it, so callers that need the
    // result immediately (the router guard) don't have to re-read the state.
    async refreshAuthUser({ commit }) {
      const authUser = await get("/user/profile")
      commit("setAuthUser", authUser)
      return authUser
    },

    ...peopleActions,

    createNew({ getters, commit, dispatch }, { folderId = null } = {}) {
      // Guests may respond to events but not create them (enforced server-side too).
      if (getters.isGuest) {
        dispatch(
          "showError",
          "Guests cannot create gatherings. Ask a member to raise your standing."
        )
        return
      }
      commit("setNewDialogOptions", {
        show: true,
        contactsPayload: {},
        folderId: folderId,
      })
    },

    // Events
    getEvents({ commit, dispatch, state }) {
      if (state.authUser) {
        return Promise.allSettled([get("/user/folders"), get("/user/events")])
          .then(([folders, events]) => {
            // Committed independently. These used to be all-or-nothing, so one
            // failing read blanked the whole dashboard even though the other
            // had arrived — and offline, where a cached copy of one may exist
            // without the other, that is the ordinary case rather than a rare
            // one.
            if (folders.status === "fulfilled") {
              commit("setFolders", folders.value)
            }
            if (events.status === "fulfilled") {
              commit("setEvents", events.value)
              // Warm the cache for every active gathering, so the one they
              // want on the train is there whether or not they happened to
              // open it. Not awaited: it runs behind the dashboard, not in
              // front of it.
              prefetchGatherings(events.value)
            }

            const failures = [folders, events]
              .filter((r) => r.status === "rejected")
              .map((r) => r.reason)
            if (failures.length === 0) return
            // Being offline is not a problem to shout about — the offline
            // banner already says so, and a snackbar reading "there was a
            // problem" over perfectly good cached data is just noise.
            if (failures.every((reason) => reason?.offline)) return

            dispatch("showError", "There was a problem fetching events!")
            console.error(...failures)
          })
          .catch((err) => {
            dispatch("showError", "There was a problem fetching events!")
            console.error(err)
          })
      } else {
        return null
      }
    },
    async archiveEvent({ dispatch, state }, { eventId, archive }) {
      try {
        await archiveEvent(eventId, archive)
        const event = state.events.find((e) => e._id === eventId)
        if (event) {
          event.isArchived = archive
        }
      } catch (err) {
        dispatch("showError", "There was a problem archiving the event!")
        console.error(err)
      }
    },
    async createFolder({ commit, dispatch }, { name, color }) {
      try {
        const folder = await createFolder(name, color)
        commit("addFolder", {
          _id: folder.id,
          name,
          color,
          eventIds: [],
        })
      } catch (err) {
        dispatch("showError", "There was a problem creating the folder!")
        console.error(err)
      }
    },
    async updateFolder({ commit, dispatch }, { folderId, name, color }) {
      try {
        await updateFolder(folderId, name, color)
        commit("updateFolder", { folderId, name, color })
      } catch (err) {
        dispatch("showError", "There was a problem updating the folder!")
        console.error(err)
      }
    },
    async deleteFolder({ commit, dispatch }, folderId) {
      try {
        await deleteFolder(folderId)
        commit("removeFolder", folderId)
      } catch (err) {
        dispatch("showError", "There was a problem deleting the folder!")
        console.error(err)
      }
    },
    async setEventFolder({ commit, dispatch }, { eventId, folderId }) {
      try {
        commit("removeEventFromFolder", eventId)
        commit("addEventToFolder", { eventId, folderId })
        await setEventFolder(eventId, folderId)
      } catch (err) {
        dispatch("showError", "There was a problem moving the event!")
        console.error(err)
      }
    },
  },
  modules: {},
})
