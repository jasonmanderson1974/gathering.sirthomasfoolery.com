<template>
  <div class="tw-mx-auto tw-mb-12 tw-mt-5 tw-max-w-3xl">
    <div class="tw-flex tw-flex-col tw-gap-8 tw-p-4">
      <!-- Header -->
      <div class="tw-flex tw-flex-col tw-gap-1">
        <div class="tw-font-head tw-text-2xl tw-text-parchment sm:tw-text-3xl">
          The Roll
        </div>
        <div class="tw-text-sm tw-text-parchment-dim">
          <span v-if="canManageUsers">
            Only those on the roll may enter the Fellowship. Extend invitations
            and set each member's standing; strike an email to revoke access.
          </span>
          <span v-else>
            Invite a guest below, and see everyone on the roll. Only an admin may
            raise a member's standing or strike them from the roll.
          </span>
        </div>
      </div>

      <!-- Invite form -->
      <div
        class="tw-rounded-xl tw-border tw-border-brass-dim tw-bg-leather/60 tw-p-4"
      >
        <div class="tw-mb-1 tw-text-sm tw-font-medium tw-text-parchment">
          Invite {{ canManageUsers ? "a member" : "a guest" }}
        </div>
        <div class="tw-flex tw-flex-col tw-gap-2 sm:tw-flex-row">
          <v-text-field
            v-model="email"
            class="tw-flex-1"
            placeholder="name@example.com"
            type="email"
            solo
            hide-details="auto"
            :error-messages="emailError"
            :disabled="adding"
            @keydown.enter="invite"
          />
          <v-select
            v-if="canManageUsers"
            v-model="inviteRole"
            :items="grantableRoleOptions"
            solo
            hide-details
            :disabled="adding"
            class="sm:tw-max-w-[10rem]"
          />
          <v-btn
            color="primary"
            :loading="adding"
            :disabled="adding"
            @click="invite"
          >
            Extend invitation
          </v-btn>
        </div>
        <div v-if="!canManageUsers" class="tw-mt-1 tw-text-xs tw-text-parchment-dim">
          Members may invite guests only.
        </div>
      </div>

      <!-- Roll — visible to all members; management controls are admin-only -->
      <div v-if="canInvite" class="tw-flex tw-flex-col tw-gap-3">
        <div class="tw-text-lg tw-font-medium tw-text-parchment">
          On the roll
          <span class="tw-text-parchment-dim">({{ members.length }})</span>
        </div>

        <div v-if="loading" class="tw-py-8 tw-text-center tw-text-parchment-dim">
          <v-progress-circular indeterminate color="brass" size="24" />
        </div>

        <div
          v-else-if="members.length === 0"
          class="tw-rounded-xl tw-border tw-border-brass-dim tw-bg-leather/40 tw-py-8 tw-text-center tw-text-sm tw-text-parchment-dim"
        >
          The roll is empty — the gate stands open until the first member is
          added.
        </div>

        <div
          v-for="member in members"
          :key="member.email"
          class="tw-flex tw-items-center tw-gap-3 tw-rounded-xl tw-border tw-border-brass-dim tw-bg-leather/40 tw-p-3"
        >
          <div class="tw-shrink-0">
            <UserAvatarContent :user="member" :size="36" />
          </div>

          <div class="tw-min-w-0 tw-flex-1">
            <div class="tw-truncate tw-font-medium tw-text-parchment">
              <span v-if="member.hasAccount">
                {{ rollDisplayName(member) }}
              </span>
              <span v-else class="tw-italic tw-text-parchment-dim">
                Awaiting first entry
              </span>
            </div>
            <div class="tw-truncate tw-text-sm tw-text-parchment-dim">
              {{ member.email }}
            </div>
          </div>

          <!-- Account status -->
          <span
            class="tw-shrink-0 tw-rounded-full tw-border tw-px-2 tw-py-0.5 tw-text-xs"
            :class="
              member.hasAccount
                ? 'tw-border-brass-dim tw-text-parchment-dim'
                : 'tw-border-brass-dim tw-text-parchment-dim tw-italic'
            "
          >
            {{ member.hasAccount ? "Joined" : "Invited" }}
          </span>

          <!-- Role: editable selector, or a locked badge for super admin / self -->
          <v-select
            v-if="isEditable(member)"
            :value="member.role"
            :items="grantableRoleOptions"
            solo
            dense
            hide-details
            :loading="busyEmail === member.email"
            :disabled="busyEmail === member.email"
            class="tw-shrink-0 tw-max-w-[9rem]"
            @change="changeRole(member, $event)"
          />
          <span
            v-else
            class="tw-shrink-0 tw-rounded-full tw-border tw-px-2 tw-py-0.5 tw-text-xs"
            :class="roleBadgeClass(member.role)"
          >
            {{ roleLabel(member.role) }}
          </span>

          <!-- Strike (admins only) -->
          <v-btn
            v-if="canManageUsers"
            icon
            small
            :disabled="!canStrike(member) || busyEmail === member.email"
            :title="strikeTitle(member)"
            @click="remove(member)"
          >
            <v-icon small color="oxblood">mdi-close</v-icon>
          </v-btn>
        </div>
      </div>

      <!-- Role reference (admins only) -->
      <div v-if="canManageUsers" class="tw-flex tw-flex-col tw-gap-3">
        <div class="tw-text-lg tw-font-medium tw-text-parchment">
          What the standings mean
        </div>
        <div
          class="tw-overflow-x-auto tw-rounded-xl tw-border tw-border-brass-dim tw-bg-leather/40"
        >
          <table class="tw-w-full tw-min-w-[560px] tw-text-sm">
            <thead>
              <tr class="tw-border-b tw-border-brass-dim">
                <th
                  class="tw-p-3 tw-text-left tw-font-medium tw-text-parchment-dim"
                >
                  Privilege
                </th>
                <th
                  v-for="col in roleMatrix.cols"
                  :key="col"
                  class="tw-p-3 tw-text-center"
                >
                  <span
                    class="tw-whitespace-nowrap tw-rounded-full tw-border tw-px-2 tw-py-0.5 tw-text-xs"
                    :class="roleBadgeClass(col)"
                    >{{ roleLabel(col) }}</span
                  >
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(row, i) in roleMatrix.rows"
                :key="i"
                :class="
                  i < roleMatrix.rows.length - 1
                    ? 'tw-border-b tw-border-brass-dim/40'
                    : ''
                "
              >
                <td class="tw-p-3 tw-text-parchment">{{ row.label }}</td>
                <td
                  v-for="(cap, j) in row.caps"
                  :key="j"
                  class="tw-p-3 tw-text-center"
                >
                  <v-icon v-if="cap" small color="brass">mdi-check</v-icon>
                  <span v-else class="tw-text-parchment-dim">—</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="tw-text-xs tw-text-parchment-dim">
          Admins may manage everyone except a Super Admin. The Super Admin
          standing is set directly in the records and cannot be granted,
          changed, or revoked through the app.
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { mapState, mapGetters, mapActions } from "vuex"
import { get, post, _delete, rollDisplayName } from "@/utils"
import { roles, roleLabels } from "@/constants"
import UserAvatarContent from "@/components/UserAvatarContent.vue"

export default {
  name: "MemberAdmin",

  components: { UserAvatarContent },

  metaInfo() {
    return { title: "The Roll · The Fellowship" }
  },

  data() {
    return {
      email: "",
      emailError: "",
      inviteRole: roles.GUEST,
      members: [],
      loading: true,
      adding: false,
      busyEmail: "",
      // Static reference chart: what each standing may do (columns ascend in privilege)
      roleMatrix: {
        cols: [roles.GUEST, roles.MEMBER, roles.ADMIN, roles.SUPER_ADMIN],
        rows: [
          { label: "Respond to gatherings", caps: [true, true, true, true] },
          { label: "Create gatherings", caps: [false, true, true, true] },
          { label: "Invite guests", caps: [false, true, true, true] },
          { label: "Invite members & admins", caps: [false, false, true, true] },
          {
            label: "Manage the roll — set standings, strike members",
            caps: [false, false, true, true],
          },
          {
            label: "Cannot be removed or demoted in the app",
            caps: [false, false, false, true],
          },
        ],
      },
    }
  },

  computed: {
    ...mapState(["authUser"]),
    ...mapGetters(["canInvite", "canManageUsers"]),
    selfEmail() {
      return (this.authUser?.email || "").toLowerCase()
    },
    // Roles this actor may grant. Admins: guest/member/admin. Members: guest only.
    grantableRoleOptions() {
      const opts = [{ text: roleLabels[roles.GUEST], value: roles.GUEST }]
      if (this.canManageUsers) {
        opts.push(
          { text: roleLabels[roles.MEMBER], value: roles.MEMBER },
          { text: roleLabels[roles.ADMIN], value: roles.ADMIN }
        )
      }
      return opts
    },
  },

  async created() {
    // Client-side guard; the /admin endpoints enforce this server-side too.
    if (!this.canInvite) {
      this.$router.replace({ name: "home" })
      return
    }
    // Members and admins alike may view the roll (read-only for members).
    await this.fetchAllowlist()
  },

  methods: {
    rollDisplayName,
    ...mapActions(["showError", "showInfo"]),
    roleLabel(role) {
      return roleLabels[role] || roleLabels[roles.MEMBER]
    },
    roleBadgeClass(role) {
      if (role === roles.SUPER_ADMIN) return "tw-border-brass tw-text-brass-bright"
      if (role === roles.ADMIN) return "tw-border-brass tw-text-brass"
      if (role === roles.GUEST) return "tw-border-brass-dim tw-text-parchment-dim"
      return "tw-border-brass-dim tw-text-parchment"
    },
    // A row's role may be changed only by an admin, and never for a super admin
    // or for the admin's own account.
    isEditable(member) {
      return (
        this.canManageUsers &&
        member.role !== roles.SUPER_ADMIN &&
        member.email.toLowerCase() !== this.selfEmail
      )
    },
    canStrike(member) {
      return (
        this.canManageUsers &&
        member.role !== roles.SUPER_ADMIN &&
        member.email.toLowerCase() !== this.selfEmail
      )
    },
    strikeTitle(member) {
      if (member.role === roles.SUPER_ADMIN)
        return "The Super Admin cannot be struck from the roll"
      if (member.email.toLowerCase() === this.selfEmail)
        return "You cannot strike yourself from the roll"
      return "Strike from the roll"
    },
    async fetchAllowlist() {
      this.loading = true
      try {
        this.members = await get("/admin/allowlist")
      } catch (err) {
        this.showError("Could not load the roll. Please try again.")
      } finally {
        this.loading = false
      }
    },
    validateEmail() {
      const email = this.email.trim()
      if (!email) {
        this.emailError = "Please enter an email address."
        return false
      }
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
        this.emailError = "Please enter a valid email address."
        return false
      }
      if (email.includes("+")) {
        this.emailError = "Email aliases with '+' are not allowed."
        return false
      }
      return true
    },
    async invite() {
      if (this.adding) return
      this.emailError = ""
      if (!this.validateEmail()) return
      this.adding = true
      try {
        const role = this.canManageUsers ? this.inviteRole : roles.GUEST
        const invitedEmail = this.email.trim()
        const res = await post("/admin/allowlist", { email: invitedEmail, role })
        this.email = ""
        this.inviteRole = roles.GUEST
        if (this.canManageUsers) await this.fetchAllowlist()
        if (res.hasAccount) {
          this.showInfo(`${res.email} is already a member — added to the roll.`)
        } else if (res.emailSent) {
          this.showInfo(`Invitation email sent to ${res.email}.`)
        } else {
          this.showError(
            `${res.email} was added to the roll, but the invitation email could not be sent.`
          )
        }
      } catch (err) {
        this.emailError = "Could not add that email. Please try again."
      } finally {
        this.adding = false
      }
    },
    async changeRole(member, role) {
      if (this.busyEmail) return
      this.busyEmail = member.email
      try {
        await post("/admin/member/role", { email: member.email, role })
        await this.fetchAllowlist()
        this.showInfo("Standing updated.")
      } catch (err) {
        this.showError("Could not update that member's standing.")
        await this.fetchAllowlist() // revert the selector
      } finally {
        this.busyEmail = ""
      }
    },
    async remove(member) {
      if (this.busyEmail) return
      if (
        !window.confirm(
          `Strike ${member.email} from the roll? They will lose access to the Fellowship.`
        )
      ) {
        return
      }
      this.busyEmail = member.email
      try {
        await _delete("/admin/allowlist", { email: member.email })
        await this.fetchAllowlist()
      } catch (err) {
        this.showError("Could not remove that email. Please try again.")
      } finally {
        this.busyEmail = ""
      }
    },
  },
}
</script>
