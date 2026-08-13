/*
 * The Fellowship, cached by account id, for the hover card (N3).
 *
 * Split out of `store/index.js` for the same reason `role_getters.js` is —
 * so the dom tier can spread the REAL implementation into its test store
 * instead of a hand-rolled stub that is free to disagree with the app. The
 * store itself stays non-modular; this is four objects that get spread in, not
 * a Vuex module.
 *
 * The two fetch paths are a privacy boundary, not an optimisation:
 * /admin/allowlist is behind CanInviteRequired and answers a guest with 403, so
 * a guest is sent to the public /users/:id instead and gets a card with names
 * and a photo but no contact details. Choosing the wrong one doesn't degrade,
 * it 403s.
 */
import { get } from "@/utils"
import { indexDirectory } from "@/utils/directory"

export const peopleState = () => ({
  // userId (hex) -> allowlist row (member+) or public profile (guest).
  people: {},
  directoryLoaded: false,
  // In-flight requests, so twenty avatars hovered in a row make one call. Held
  // in state rather than at module scope so two stores — every dom test builds
  // its own — cannot share one another's fetches. Vue leaves a Promise
  // un-proxied (it is not one of the types `reactive()` wraps), so awaiting
  // what comes back out of here is safe.
  directoryRequest: null,
  peopleRequests: {},
})

export const peopleGetters = {
  // Null rather than undefined for "not held", so a caller can tell "no record"
  // from "a record with nothing in it".
  personById: (state) => (userId) => state.people[userId] ?? null,
}

export const peopleMutations = {
  // Merged, never replaced: the guest path fills this one id at a time and a
  // second lookup must not drop the first.
  addPeople(state, byId) {
    state.people = { ...state.people, ...byId }
  },
  setDirectoryLoaded(state, loaded) {
    state.directoryLoaded = loaded
  },
  setDirectoryRequest(state, request) {
    state.directoryRequest = request
  },
  setPersonRequest(state, { userId, request }) {
    if (request) state.peopleRequests[userId] = request
    else delete state.peopleRequests[userId]
  },
}

export const peopleActions = {
  /*
   * Make sure the card has something to show for `userId`.
   *
   * Nothing here rejects. It is called from a mouseenter; a failure means the
   * card opens with only what the page already had, which is the same thing it
   * does while the request is still in flight. An error snackbar because
   * someone moved the mouse would be worse than the missing phone number.
   */
  ensurePerson({ state, getters, dispatch }, userId) {
    if (!userId) return Promise.resolve()
    if (getters.canInvite) return dispatch("ensureDirectory")
    if (state.people[userId] || state.peopleRequests[userId]) {
      return Promise.resolve()
    }
    return dispatch("fetchPublicProfile", userId)
  },

  // One request for the whole club, ever — the same call /fellowship and
  // /members already make, now made once for the session instead of per view.
  ensureDirectory({ state, commit }) {
    if (state.directoryLoaded) return Promise.resolve()
    if (state.directoryRequest) return state.directoryRequest

    const request = get("/admin/allowlist")
      .then((rows) => {
        commit("addPeople", indexDirectory(rows))
        commit("setDirectoryLoaded", true)
      })
      .catch(() => {
        // Left unloaded on purpose, so the next hover tries again.
      })
      .finally(() => commit("setDirectoryRequest", null))

    commit("setDirectoryRequest", request)
    return request
  },

  // The guest path. A miss is memoized as an empty record rather than left
  // absent, so hovering a deleted account repeatedly doesn't refetch a 404
  // every time.
  fetchPublicProfile({ commit }, userId) {
    const request = get(`/users/${userId}`)
      .then((user) => commit("addPeople", { [userId]: user ?? {} }))
      .catch(() => commit("addPeople", { [userId]: {} }))
      .finally(() => commit("setPersonRequest", { userId, request: null }))

    commit("setPersonRequest", { userId, request })
    return request
  },
}
