<template>
  <div class="tw-mx-auto tw-mb-12 tw-mt-5 tw-max-w-3xl">
    <div class="tw-flex tw-flex-col tw-gap-6 tw-p-4">
      <!-- Header -->
      <div class="tw-flex tw-flex-col tw-gap-1">
        <div class="tw-font-head tw-text-2xl tw-text-parchment sm:tw-text-3xl">
          The Fellowship
        </div>
        <div class="tw-text-sm tw-text-parchment-dim">
          A directory of the membership — reach a fellow gentleman by post or by
          telephone.
        </div>
      </div>

      <!-- Controls -->
      <div
        class="tw-flex tw-flex-col tw-gap-3 sm:tw-flex-row sm:tw-items-center sm:tw-justify-between"
      >
        <v-text-field
          v-model="search"
          class="tw-flex-1"
          placeholder="Search by name or email..."
          variant="solo"
          hide-details
          clearable
          prepend-inner-icon="mdi-magnify"
          :dense="isPhone"
        />
        <div class="tw-flex tw-shrink-0 tw-items-center tw-gap-3">
          <v-switch
            v-model="showGuests"
            color="brass"
            hide-details
            dense
            class="tw-mt-0 tw-shrink-0 tw-pt-0"
            label="Show guests"
          />
          <v-menu>
            <template v-slot:activator="{ props }">
              <v-btn
                id="roster-export-btn"
                variant="outlined"
                size="small"
                class="tw-text-brass"
                :disabled="filteredMembers.length === 0"
                v-bind="props"
              >
                <v-icon size="small" left>mdi-download</v-icon>
                Export
              </v-btn>
            </template>
            <v-list density="compact">
              <v-list-item id="roster-print-btn" @click="printRoster">
                <v-list-item-title class="tw-flex tw-items-center tw-gap-2">
                  <v-icon size="small">mdi-printer</v-icon>
                  Print / PDF
                </v-list-item-title>
              </v-list-item>
              <v-list-item id="roster-csv-btn" @click="exportCsv">
                <v-list-item-title class="tw-flex tw-items-center tw-gap-2">
                  <v-icon size="small">mdi-file-delimited-outline</v-icon>
                  Download CSV
                </v-list-item-title>
              </v-list-item>
            </v-list>
          </v-menu>
        </div>
      </div>

      <div v-if="loading" class="tw-py-10 tw-text-center tw-text-parchment-dim">
        <v-progress-circular indeterminate color="brass" size="24" />
      </div>

      <div
        v-else-if="filteredMembers.length === 0"
        class="tw-rounded-xl tw-border tw-border-brass-dim tw-bg-leather/40 tw-py-10 tw-text-center tw-text-sm tw-text-parchment-dim"
      >
        {{
          search ? "No one on the roll matches your search." : "No one to show."
        }}
      </div>

      <!-- Directory -->
      <div v-else class="tw-flex tw-flex-col tw-gap-3">
        <div class="tw-text-sm tw-text-parchment-dim">
          {{ filteredMembers.length }}
          {{ filteredMembers.length === 1 ? "gentleman" : "gentlemen" }}
        </div>
        <div class="tw-grid tw-grid-cols-1 tw-gap-3 sm:tw-grid-cols-2">
          <div
            v-for="member in filteredMembers"
            :key="member.email"
            class="tw-flex tw-gap-3 tw-rounded-xl tw-border tw-border-brass-dim tw-bg-leather/40 tw-p-4"
          >
            <!-- Photo, or the monogram this card used to draw itself -->
            <div class="tw-shrink-0">
              <UserAvatarContent :user="member" :size="40" />
            </div>

            <div class="tw-min-w-0 tw-flex-1">
              <div class="tw-flex tw-items-center tw-gap-2">
                <span
                  v-if="member.hasAccount"
                  class="tw-truncate tw-font-medium tw-text-parchment"
                >
                  {{ rollDisplayName(member) }}
                </span>
                <span
                  v-else
                  class="tw-truncate tw-italic tw-text-parchment-dim"
                >
                  Awaiting first entry
                </span>
                <span
                  class="tw-shrink-0 tw-rounded-full tw-border tw-px-2 tw-py-0.5 tw-text-xs"
                  :class="roleBadgeClass(member.role)"
                >
                  {{ roleLabel(member.role) }}
                </span>
              </div>

              <!-- Email -->
              <a
                :href="`mailto:${member.email}`"
                class="tw-mt-1 tw-flex tw-items-center tw-gap-1 tw-truncate tw-text-sm tw-text-parchment-dim hover:tw-text-brass"
              >
                <v-icon size="x-small" color="brass">mdi-email-outline</v-icon>
                <span class="tw-truncate">{{ member.email }}</span>
              </a>

              <!-- Phone -->
              <a
                v-if="member.phone"
                :href="`tel:${member.phone}`"
                class="tw-mt-1 tw-flex tw-items-center tw-gap-1 tw-text-sm tw-text-parchment-dim hover:tw-text-brass"
              >
                <v-icon size="x-small" color="brass">mdi-phone-outline</v-icon>
                <span>{{ formatPhone(member.phone) }}</span>
              </a>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { mapGetters } from "vuex"
import { get, formatPhone, isPhone, rollDisplayName } from "@/utils"
import { roles, roleLabels } from "@/constants"
import UserAvatarContent from "@/components/UserAvatarContent.vue"

export default {
  name: "Fellowship",

  components: { UserAvatarContent },

  metaInfo() {
    return { title: "The Fellowship · Directory" }
  },

  data() {
    return {
      members: [],
      loading: true,
      search: "",
      showGuests: true,
    }
  },

  computed: {
    ...mapGetters(["canInvite"]),
    isPhone() {
      return isPhone(this.$vuetify)
    },
    filteredMembers() {
      const q = (this.search || "").trim().toLowerCase()
      return this.members.filter((m) => {
        if (!this.showGuests && m.role === roles.GUEST) return false
        if (!q) return true
        // Search both names — someone looking for "Bartholomew" should find
        // him even when the roll shows "Bart".
        const hay = `${m.nickname || ""} ${m.firstName || ""} ${
          m.lastName || ""
        } ${m.email}`.toLowerCase()
        return hay.includes(q)
      })
    },
  },

  async created() {
    // Directory is for member+ (guests are excluded by the /admin group too).
    if (!this.canInvite) {
      this.$router.replace({ name: "home" })
      return
    }
    try {
      this.members = await get("/admin/allowlist")
    } catch (err) {
      this.members = []
    } finally {
      this.loading = false
    }
  },

  methods: {
    rollDisplayName,
    formatPhone,
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
    // Display name for exports (matches the card: pending entries show a
    // placeholder rather than a blank).
    exportName(m) {
      const name = rollDisplayName(m)
      return m.hasAccount && name ? name : "Awaiting first entry"
    },
    // Download the currently-filtered roll as a CSV (what you see is what you
    // get — respects the search + Show-guests filter).
    exportCsv() {
      const esc = (v) => {
        const s = String(v ?? "")
        return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s
      }
      const rows = [["Name", "Role", "Email", "Phone"]]
      for (const m of this.filteredMembers) {
        rows.push([
          this.exportName(m),
          this.roleLabel(m.role),
          m.email || "",
          m.phone ? formatPhone(m.phone) : "",
        ])
      }
      const csv = rows.map((r) => r.map(esc).join(",")).join("\n")
      const blob = new Blob([csv], { type: "text/csv;charset=utf-8" })
      const url = URL.createObjectURL(blob)
      const a = document.createElement("a")
      a.href = url
      a.download = "The Fellowship Roster.csv"
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
    },
    // Open a clean, light, print-friendly window with the filtered roll and
    // trigger the browser print dialog (from which members can Save as PDF).
    // A separate document avoids fighting the app's dark theme + chrome.
    printRoster() {
      const esc = (s) =>
        String(s ?? "").replace(
          /[&<>]/g,
          (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;" }[c])
        )
      const today = new Date().toLocaleDateString(undefined, {
        year: "numeric",
        month: "long",
        day: "numeric",
      })
      const body = this.filteredMembers
        .map(
          (m) =>
            `<tr><td>${esc(this.exportName(m))}</td><td>${esc(
              this.roleLabel(m.role)
            )}</td><td>${esc(m.email)}</td><td>${esc(
              m.phone ? formatPhone(m.phone) : ""
            )}</td></tr>`
        )
        .join("")
      const html = `<!doctype html><html><head><meta charset="utf-8" />
<title>The Fellowship — Directory</title>
<style>
  * { box-sizing: border-box; }
  body { font-family: Georgia, "Times New Roman", serif; color: #1a1a1a; margin: 2rem; }
  h1 { font-size: 1.6rem; margin: 0 0 0.25rem; }
  .sub { color: #666; font-size: 0.85rem; margin-bottom: 1.25rem; }
  table { width: 100%; border-collapse: collapse; font-size: 0.9rem; }
  th, td { text-align: left; padding: 0.5rem 0.6rem; border-bottom: 1px solid #ccc; }
  th { border-bottom: 2px solid #333; font-variant: small-caps; letter-spacing: 0.03em; }
  tr:nth-child(even) td { background: #f6f4ef; }
  @media print { body { margin: 0.75rem; } }
</style></head><body>
<h1>The Fellowship</h1>
<div class="sub">A directory of the membership · ${esc(
        String(this.filteredMembers.length)
      )} ${
        this.filteredMembers.length === 1 ? "gentleman" : "gentlemen"
      } · ${esc(today)}</div>
<table><thead><tr><th>Name</th><th>Role</th><th>Email</th><th>Telephone</th></tr></thead>
<tbody>${body}</tbody></table>
</body></html>`
      const w = window.open("", "_blank")
      if (!w) return
      w.document.write(html)
      w.document.close()
      w.focus()
      w.print()
    },
  },
}
</script>
