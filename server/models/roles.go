package models

// Role is a user's access level in the invite-only Fellowship. Ordered by
// privilege: superAdmin > admin > member > guest.
type Role string

const (
	RoleSuperAdmin Role = "superAdmin"
	RoleAdmin      Role = "admin"
	RoleMember     Role = "member"
	RoleGuest      Role = "guest"
)

// IsKnownRole reports whether r is one of the defined roles (rejects
// empty/garbage input at the API boundary).
func IsKnownRole(r Role) bool {
	switch r {
	case RoleSuperAdmin, RoleAdmin, RoleMember, RoleGuest:
		return true
	default:
		return false
	}
}

// NormalizeRole coerces an empty/unknown role to the default (member). New
// invited guests always carry an explicit "guest", so an empty role only
// happens for legacy accounts, which are treated as members.
func NormalizeRole(r Role) Role {
	switch r {
	case RoleSuperAdmin, RoleAdmin, RoleMember, RoleGuest:
		return r
	default:
		return RoleMember
	}
}

// rank orders roles for comparison (higher = more privilege).
func (r Role) rank() int {
	switch NormalizeRole(r) {
	case RoleSuperAdmin:
		return 3
	case RoleAdmin:
		return 2
	case RoleMember:
		return 1
	default: // guest
		return 0
	}
}

// AtLeast reports whether r has at least the privilege of other.
func (r Role) AtLeast(other Role) bool { return r.rank() >= other.rank() }

// CanManageUsers: admin and superAdmin may add/remove/change roles of members.
func (r Role) CanManageUsers() bool { return r.AtLeast(RoleAdmin) }

// CanInvite: member and up may add emails to the allowlist (members: guests only).
func (r Role) CanInvite() bool { return r.AtLeast(RoleMember) }

// CanSeeMembersOnly: member and up may read (and post in) members-only
// discussion threads, and may tag a comment as a thread. Guests are excluded.
func (r Role) CanSeeMembersOnly() bool { return r.AtLeast(RoleMember) }

// CanCreateEvents: everyone except guests.
func (r Role) CanCreateEvents() bool { return NormalizeRole(r) != RoleGuest }

// IsSuperAdmin: the immutable top role (assigned only via the database).
func (r Role) IsSuperAdmin() bool { return NormalizeRole(r) == RoleSuperAdmin }

// EffectiveRole returns the user's normalized role.
func (u *User) EffectiveRole() Role { return NormalizeRole(u.Role) }
