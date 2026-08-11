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
            Invite a guest below, and see everyone on the roll. Only an admin
            may raise a member's standing or strike them from the roll.
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
            variant="solo"
            hide-details="auto"
            :error-messages="emailError"
            :disabled="adding"
            @keydown.enter="invite"
          />
          <v-select
            v-if="canManageUsers"
            v-model="inviteRole"
            :items="grantableRoleOptions"
            item-title="text"
            variant="solo"
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
        <div
          v-if="!canManageUsers"
          class="tw-mt-1 tw-text-xs tw-text-parchment-dim"
        >
          Members may invite guests only.
        </div>
      </div>

      <!-- Roll — visible to all members; management controls are admin-only -->
      <div v-if="canInvite" class="tw-flex tw-flex-col tw-gap-3">
        <div class="tw-text-lg tw-font-medium tw-text-parchment">
          On the roll
          <span class="tw-text-parchment-dim">({{ members.length }})</span>
        </div>

        <div
          v-if="loading"
          class="tw-py-8 tw-text-center tw-text-parchment-dim"
        >
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
                : 'tw-border-brass-dim tw-italic tw-text-parchment-dim'
            "
          >
            {{ member.hasAccount ? "Joined" : "Invited" }}
          </span>

          <!-- Role: editable selector, or a locked badge for super admin / self -->
          <v-select
            v-if="isEditable(member)"
            :model-value="member.role"
            :items="grantableRoleOptions"
            item-title="text"
            variant="solo"
            density="compact"
            hide-details
            :loading="busyEmail === member.email"
            :disabled="busyEmail === member.email"
            class="tw-max-w-[9rem] tw-shrink-0"
            @change="changeRole(member, $event)"
          />
          <span
            v-else
            class="tw-shrink-0 tw-rounded-full tw-border tw-px-2 tw-py-0.5 tw-text-xs"
            :class="roleBadgeClass(member.role)"
          >
            {{ roleLabel(member.role) }}
          </span>

          <!-- Edit name/nickname/photo (admins only, and only once they've joined) -->
          <v-btn
            v-if="canEditProfile(member)"
            icon
            size="small"
            :disabled="busyEmail === member.email"
            title="Edit name, nickname and photo"
            @click="openEditor(member)"
          >
            <v-icon size="small" color="brass">mdi-pencil</v-icon>
          </v-btn>

          <!-- Strike (admins only) -->
          <v-btn
            v-if="canManageUsers"
            icon
            size="small"
            :disabled="!canStrike(member) || busyEmail === member.email"
            :title="strikeTitle(member)"
            @click="remove(member)"
          >
            <v-icon size="small" color="oxblood">mdi-close</v-icon>
          </v-btn>
        </div>
      </div>

      <!--
        Profile editor. Bound to `editEmail` rather than to a copied row, so
        `editingMember` re-resolves out of `members` after a re-fetch — that is
        what refreshes the photo preview in place once an upload lands.
      -->
      <v-dialog v-model="editDialog" width="480" content-class="tw-m-0">
        <v-card v-if="editingMember">
          <v-card-title class="tw-font-head">
            Edit {{ rollDisplayName(editingMember) }}
          </v-card-title>
          <v-card-text>
            <div class="tw-mb-4 tw-text-sm tw-text-parchment-dim">
              {{ editingMember.email }}
            </div>

            <!-- Photo -->
            <div class="tw-mb-6 tw-flex tw-items-center tw-gap-4">
              <UserAvatarContent :user="editingMember" :size="72" />
              <div class="tw-flex tw-flex-col tw-items-start tw-gap-1">
                <v-btn
                  variant="text"
                  size="small"
                  class="tw-text-brass"
                  :loading="savingAvatar"
                  @click="$refs.avatarEditor.pickFile()"
                >
                  {{
                    editingMember.avatarUpdatedAt ? "Change photo" : "Add photo"
                  }}
                </v-btn>
                <v-btn
                  v-if="editingMember.avatarUpdatedAt"
                  variant="text"
                  size="small"
                  :loading="removingAvatar"
                  @click="removeMemberAvatar"
                >
                  Remove photo
                </v-btn>
              </div>
            </div>

            <!--
              Captions above the fields rather than Vuetify `label` props:
              `solo` hides the label as soon as the field has a value, which is
              always here (the form opens pre-filled), leaving unlabelled boxes.
              Settings uses the same heading-above-field shape.
            -->
            <div class="tw-mb-1 tw-text-sm tw-font-medium tw-text-parchment">
              First name
            </div>
            <v-text-field
              v-model="editFirstName"
              variant="solo"
              hide-details="auto"
              class="tw-mb-3"
              :disabled="savingProfile"
            />
            <div class="tw-mb-1 tw-text-sm tw-font-medium tw-text-parchment">
              Last name
            </div>
            <v-text-field
              v-model="editLastName"
              variant="solo"
              hide-details="auto"
              class="tw-mb-3"
              :disabled="savingProfile"
            />
            <div class="tw-mb-1 tw-text-sm tw-font-medium tw-text-parchment">
              Nickname
            </div>
            <v-text-field
              v-model="editNickname"
              placeholder="What the club actually calls them"
              variant="solo"
              hide-details="auto"
              :disabled="savingProfile"
            />
            <div class="tw-mt-1 tw-text-xs tw-text-parchment-dim">
              A nickname stands in for the name wherever it appears. Leave it
              empty to remove one.
            </div>
          </v-card-text>

          <v-card-actions>
            <v-spacer />
            <v-btn
              variant="text"
              class="tw-text-brass"
              @click="editDialog = false"
            >
              Cancel
            </v-btn>
            <v-btn variant="text" :loading="savingProfile" @click="saveProfile">
              Save
            </v-btn>
          </v-card-actions>
        </v-card>
      </v-dialog>

      <!--
        One editor instance for the whole roll, outside the v-for: it is only
        ever open for `editingMember`, and a per-row copy would mean a hidden
        file input per member.

        The title uses displayName, not rollDisplayName: the
        "Nickname (First Last)" pairing is right for scanning a list, but in a
        heading it wraps mid-word.
      -->
      <AvatarEditorDialog
        ref="avatarEditor"
        v-model="avatarDialog"
        :saving="savingAvatar"
        :title="
          editingMember
            ? `Choose a photo for ${displayName(editingMember)}`
            : 'Choose a photo'
        "
        @save="saveMemberAvatar"
      />

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
                  <v-icon v-if="cap" size="small" color="brass"
                    >mdi-check</v-icon
                  >
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
import {
  get,
  post,
  patch,
  put,
  _delete,
  displayName,
  rollDisplayName,
} from "@/utils"
import { roles, roleLabels } from "@/constants"
import UserAvatarContent from "@/components/UserAvatarContent.vue"
import AvatarEditorDialog from "@/components/settings/AvatarEditorDialog.vue"

export default {
  name: "MemberAdmin",

  components: { UserAvatarContent, AvatarEditorDialog },

  head() {
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
      // Profile editor. The dialog keys off the email rather than a copied row
      // so it follows the member across a re-fetch.
      editDialog: false,
      editEmail: "",
      editFirstName: "",
      editLastName: "",
      editNickname: "",
      savingProfile: false,
      avatarDialog: false,
      savingAvatar: false,
      removingAvatar: false,
      // Static reference chart: what each standing may do (columns ascend in privilege)
      roleMatrix: {
        cols: [roles.GUEST, roles.MEMBER, roles.ADMIN, roles.SUPER_ADMIN],
        rows: [
          { label: "Respond to gatherings", caps: [true, true, true, true] },
          { label: "Create gatherings", caps: [false, true, true, true] },
          { label: "Invite guests", caps: [false, true, true, true] },
          {
            label: "Invite members & admins",
            caps: [false, false, true, true],
          },
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
    // Resolved out of `members` on every render, so a re-fetch after an avatar
    // upload updates the preview (and its `?v=`) without any extra plumbing.
    editingMember() {
      if (!this.editEmail) return null
      return this.members.find((m) => m.email === this.editEmail) || null
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
    displayName,
    ...mapActions(["showError", "showInfo"]),
    roleLabel(role) {
      return roleLabels[role] || roleLabels[roles.MEMBER]
    },
    roleBadgeClass(role) {
      if (role === roles.SUPER_ADMIN)
        return "tw-border-brass tw-text-brass-bright"
      if (role === roles.ADMIN) return "tw-border-brass tw-text-brass"
      if (role === roles.GUEST)
        return "tw-border-brass-dim tw-text-parchment-dim"
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
    // Editing writes to a user document, so an unclaimed invitation has nothing
    // to edit — the server returns 400 for that case and this hides the control.
    // Self-edit IS allowed (unlike role changes): it is harmless, and Settings
    // offers the same thing.
    canEditProfile(member) {
      return (
        this.canManageUsers &&
        member.hasAccount &&
        member.role !== roles.SUPER_ADMIN
      )
    },
    openEditor(member) {
      this.editEmail = member.email
      this.editFirstName = member.firstName || ""
      this.editLastName = member.lastName || ""
      this.editNickname = member.nickname || ""
      this.editDialog = true
    },
    async saveProfile() {
      const member = this.editingMember
      if (!member || this.savingProfile) return
      this.savingProfile = true
      try {
        // Every field is sent, so the server's "omitted means leave alone"
        // semantics are not load-bearing here — but an empty nickname still
        // means clear, which is exactly what the empty field should do.
        await patch("/admin/member/profile", {
          email: member.email,
          firstName: this.editFirstName,
          lastName: this.editLastName,
          nickname: this.editNickname,
        })
        await this.fetchAllowlist()
        this.editDialog = false
        this.showInfo("Details updated.")
      } catch (err) {
        // The one rejection worth naming: a name may be edited but not erased.
        if (err?.error === "invalid-name") {
          this.showError("A first and last name are both required.")
        } else {
          this.showError("Could not update those details.")
        }
      } finally {
        this.savingProfile = false
      }
    },
    async saveMemberAvatar(image) {
      const member = this.editingMember
      if (!member) return
      this.savingAvatar = true
      try {
        await put("/admin/member/avatar", { email: member.email, image })
        // Re-fetch rather than patch the row: avatarUpdatedAt is the cache
        // buster every avatar on screen is keyed off, so the roll needs the
        // server's new value, not a guess.
        await this.fetchAllowlist()
        this.avatarDialog = false
        this.showInfo("Photo updated.")
      } catch (err) {
        this.showError("There was a problem saving that photo.")
      } finally {
        this.savingAvatar = false
      }
    },
    async removeMemberAvatar() {
      const member = this.editingMember
      if (!member) return
      this.removingAvatar = true
      try {
        await _delete("/admin/member/avatar", { email: member.email })
        await this.fetchAllowlist()
        this.showInfo("Photo removed.")
      } catch (err) {
        this.showError("There was a problem removing that photo.")
      } finally {
        this.removingAvatar = false
      }
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
        const res = await post("/admin/allowlist", {
          email: invitedEmail,
          role,
        })
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
