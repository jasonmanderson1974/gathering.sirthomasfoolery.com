import { roles, normalizeRole } from "@/constants"

// Role-derived Vuex getters, kept in a standalone (Vue/Vuex-free) module so they
// can be unit-tested as plain functions. Spread into the store's getters.
export const roleGetters = {
  // Effective role of the signed-in user (empty ⇒ member; null user ⇒ not signed in)
  role(state) {
    return state.authUser ? normalizeRole(state.authUser.role) : null
  },
  isGuest(state, getters) {
    return getters.role === roles.GUEST
  },
  isSuperAdmin(state, getters) {
    return getters.role === roles.SUPER_ADMIN
  },
  // Can add emails to the allowlist (member and up)
  canInvite(state, getters) {
    return [roles.MEMBER, roles.ADMIN, roles.SUPER_ADMIN].includes(getters.role)
  },
  // Can manage users / change roles (admin and up)
  canManageUsers(state, getters) {
    return [roles.ADMIN, roles.SUPER_ADMIN].includes(getters.role)
  },
  // Can read members-only discussion threads and tag a comment as a thread
  // (member and up). Same predicate as canInvite today, but a distinct concept —
  // kept separate so either rule can move without dragging the other with it.
  canSeeMembersOnly(state, getters) {
    return [roles.MEMBER, roles.ADMIN, roles.SUPER_ADMIN].includes(getters.role)
  },
  // Members and up may create events. Anonymous used to be allowed here — the
  // backend accepted ownerless events and the visitor was prompted to sign in
  // on save — but E3 removed anonymous creation entirely, so `role` is null
  // for anyone not signed in and they cannot create.
  canCreateEvents(state, getters) {
    return getters.role !== null && getters.role !== roles.GUEST
  },
}
