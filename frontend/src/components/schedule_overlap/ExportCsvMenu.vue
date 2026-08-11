<template>
  <v-menu right offset-x>
    <template v-slot:activator="{ props }">
      <v-btn icon v-bind="props"><v-icon>mdi-dots-vertical</v-icon></v-btn>
    </template>
    <v-list class="tw-py-1" dense>
      <v-dialog v-model="exportCsvDialog.visible" width="400">
        <template v-slot:activator="{ props }">
          <v-list-item id="export-csv-btn" v-bind="props">
            <v-list-item-title>Export CSV</v-list-item-title>
          </v-list-item>
        </template>
        <v-card>
          <v-card-title>Export CSV</v-card-title>
          <v-card-text>
            <div class="tw-mb-1">Select CSV format:</div>
            <v-select
              v-model="exportCsvDialog.type"
              solo
              hide-details
              :items="exportCsvDialog.types"
              item-title="text"
              item-value="value"
            />
          </v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn
              text
              @click="exportCsvDialog.visible = false"
              :disabled="exportCsvDialog.loading"
              >Cancel</v-btn
            >
            <v-btn
              text
              @click="exportCsv"
              color="primary"
              :loading="exportCsvDialog.loading"
              >Export</v-btn
            >
          </v-card-actions>
        </v-card>
      </v-dialog>
    </v-list>
  </v-menu>
</template>

<script>
import { getLocale, displayName } from "@/utils"

/**
 * "Export CSV" kebab menu + format dialog for the respondents list, with the
 * CSV-building/download logic. Extracted from RespondentsList.vue (TODO A11,
 * Tier 2). The parent gates rendering with its allowExportCsv computed.
 */
export default {
  name: "ExportCsvMenu",

  props: {
    event: { type: Object, required: true },
    eventId: { type: String, required: true },
    respondents: { type: Array, required: true },
    parsedResponses: { type: Object, required: true },
    timezone: { type: Object, required: true },
  },

  data() {
    return {
      exportCsvDialog: {
        visible: false,
        loading: false,
        type: "datesToAvailable",
        types: [
          {
            text: "Dates <> people available",
            value: "datesToAvailable",
          },
          { text: "Name <> dates available", value: "nameToDates" },
        ],
      },
    }
  },

  methods: {
    getDateString(date) {
      const locale = getLocale()

      if (this.event.daysOnly) {
        return date.toISOString().substring(0, 10)
      }
      return (
        '"' +
        date.toLocaleString(locale, { timeZone: this.timezone.value }) +
        '"'
      )
    },
    async exportCsv() {
      const csv = []
      const increment = 15
      const numIterations = this.event.daysOnly
        ? 1
        : (this.event.duration * 60) / increment

      // Get responses sorted by the name the export will show
      const responses = Object.values(this.parsedResponses).sort((a, b) =>
        displayName(a.user).localeCompare(displayName(b.user))
      )

      if (this.exportCsvDialog.type === "datesToAvailable") {
        // Write CSV header
        const header = ["Date / Time"]
        header.push(...responses.map((r) => displayName(r.user)))
        csv.push(header)

        // Iterate through the dates
        for (const date of this.event.dates) {
          const curDate = new Date(date)

          // Iterate through the timeslots for the current date
          for (let i = 0; i < numIterations; ++i) {
            const row = [this.getDateString(curDate)]

            // Iterate through the responses and mark whether they are available or not
            for (const response of responses) {
              if (response.availability.has(curDate.getTime())) {
                row.push("Available")
              } else if (response.ifNeeded.has(curDate.getTime())) {
                row.push("If needed")
              } else {
                row.push("")
              }
            }

            // Add row to CSV
            csv.push(row)

            // Increment curDate by the selected amount
            curDate.setMinutes(curDate.getMinutes() + increment)
          }
        }
      } else if (this.exportCsvDialog.type === "nameToDates") {
        // Write CSV header
        csv.push(["Name", "Date / Times available"])

        // Iterate through the responses
        for (const response of responses) {
          // The first row is the name
          const row = [displayName(response.user)]

          // Iterate through the dates
          for (const date of this.event.dates) {
            const curDate = new Date(date)

            // Iterate through the timeslots for the current date
            for (let i = 0; i < numIterations; ++i) {
              // If the user is available for the current timeslot, add the date to the row
              if (
                response.availability.has(curDate.getTime()) ||
                response.ifNeeded.has(curDate.getTime())
              ) {
                row.push(this.getDateString(curDate))
              } else {
                row.push("")
              }

              // Increment curDate by the selected amount
              curDate.setMinutes(curDate.getMinutes() + increment)
            }
          }
          csv.push(row)
        }
      }

      // Create CSV uri
      // Source: https://stackoverflow.com/questions/14964035/how-to-export-javascript-array-info-to-csv-on-client-side
      const csvString =
        "data:text/csv;charset=utf-8," + csv.map((e) => e.join(",")).join("\n")
      const encodedUri = encodeURI(csvString)

      // Set CSV filename and download
      // Source: https://stackoverflow.com/questions/7034754/how-to-set-a-file-name-using-window-open
      const downloadLink = document.createElement("a")
      downloadLink.href = encodedUri
      downloadLink.download = `${this.event.name}.csv`
      document.body.appendChild(downloadLink)
      downloadLink.click()
      document.body.removeChild(downloadLink)
    },
  },
}
</script>
