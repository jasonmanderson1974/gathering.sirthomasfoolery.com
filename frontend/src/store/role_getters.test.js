import { describe, test, expect } from "vitest"
import { roleGetters } from "@/store/role_getters"
import { roles } from "@/constants"

// Evaluate all role getters for a given authUser, resolving the inter-getter
// dependencies (they read `getters.role`) in order — same as Vuex would.
function evalFor(authUser) {
  const state = { authUser }
  const g = {}
  g.role = roleGetters.role(state)
  g.isGuest = roleGetters.isGuest(state, g)
  g.isSuperAdmin = roleGetters.isSuperAdmin(state, g)
  g.canInvite = roleGetters.canInvite(state, g)
  g.canManageUsers = roleGetters.canManageUsers(state, g)
  g.canCreateEvents = roleGetters.canCreateEvents(state, g)
  g.canSeeMembersOnly = roleGetters.canSeeMembersOnly(state, g)
  return g
}

describe("store role getters", () => {
  test("anonymous (not signed in)", () => {
    const g = evalFor(null)
    expect(g.role).toBe(null)
    expect(g.isGuest).toBe(false)
    expect(g.isSuperAdmin).toBe(false)
    expect(g.canInvite).toBe(false)
    expect(g.canManageUsers).toBe(false)
    // anon may create (matches backend, which only blocks signed-in guests)
    expect(g.canCreateEvents).toBe(true)
    // ...but never sees members-only threads (the server sends them no
    // comments at all — the discussion is sign-in-only)
    expect(g.canSeeMembersOnly).toBe(false)
  })

  test("guest", () => {
    const g = evalFor({ role: "guest" })
    expect(g.role).toBe(roles.GUEST)
    expect(g.isGuest).toBe(true)
    expect(g.canInvite).toBe(false)
    expect(g.canManageUsers).toBe(false)
    expect(g.canCreateEvents).toBe(false)
    // the whole point of members-only threads
    expect(g.canSeeMembersOnly).toBe(false)
  })

  test("member", () => {
    const g = evalFor({ role: "member" })
    expect(g.role).toBe(roles.MEMBER)
    expect(g.isGuest).toBe(false)
    expect(g.canInvite).toBe(true)
    expect(g.canManageUsers).toBe(false)
    expect(g.canCreateEvents).toBe(true)
    expect(g.canSeeMembersOnly).toBe(true)
  })

  test("empty role is treated as member", () => {
    const g = evalFor({ role: "" })
    expect(g.role).toBe(roles.MEMBER)
    expect(g.canInvite).toBe(true)
    expect(g.canManageUsers).toBe(false)
    expect(g.canCreateEvents).toBe(true)
    expect(g.canSeeMembersOnly).toBe(true)
  })

  test("unknown role is treated as member", () => {
    const g = evalFor({ role: "wizard" })
    expect(g.role).toBe(roles.MEMBER)
    expect(g.canInvite).toBe(true)
    expect(g.canManageUsers).toBe(false)
    expect(g.canSeeMembersOnly).toBe(true)
  })

  test("admin", () => {
    const g = evalFor({ role: "admin" })
    expect(g.role).toBe(roles.ADMIN)
    expect(g.isSuperAdmin).toBe(false)
    expect(g.canInvite).toBe(true)
    expect(g.canManageUsers).toBe(true)
    expect(g.canCreateEvents).toBe(true)
    expect(g.canSeeMembersOnly).toBe(true)
  })

  test("super admin", () => {
    const g = evalFor({ role: "superAdmin" })
    expect(g.role).toBe(roles.SUPER_ADMIN)
    expect(g.isSuperAdmin).toBe(true)
    expect(g.canInvite).toBe(true)
    expect(g.canManageUsers).toBe(true)
    expect(g.canCreateEvents).toBe(true)
    expect(g.canSeeMembersOnly).toBe(true)
  })
})
