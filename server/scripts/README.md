# One-off migration scripts

Each dated folder here is a **one-off data migration**, run manually **once**
around the date in its name (`YYYYMMDD_short_description`), against the
production Mongo database. They are historical records, not part of the running
server.

## Important

- **Do not re-run these.** Several are destructive or assume a specific
  historical data shape; re-running one against current data can corrupt it.
- **They intentionally do not compile as part of the build.** Each is a
  standalone `package main` that references model shapes as they existed at the
  time, so they drift from the current `models/` package. Backend CI compiles
  only the server (`go build .`) and excludes this directory — see
  `.github/workflows/backend-ci.yml` and the `scripts` skip in
  `.golangci.yml`. Don't try to make them build again.
- **This directory is the reason every backend command is written as
  `go list ./... | grep -v '/scripts'`** rather than plain `./...` — build, vet,
  lint and test all derive their package list that way (TODO E12). If you ever
  do decide to delete these, that exclusion can go with them; until then, adding
  a folder here needs no change anywhere else.
- **Adding a new migration:** follow the same pattern — a new
  `YYYYMMDD_short_description/` folder with its own `main.go` — run it manually,
  then leave it as-is for history.

## Folders (oldest → newest, by folder-name date)

Undated folders (`add_event_type`, `new_date_representation`) predate the dated
convention. The dated ones, in order:

- `20230812_add_calendar_accounts`
- `20230914_15_minute_increments`
- `20240518_rename_blind_avail_field`
- `20240721_apple_calendar_test`
- `20240723_multiple_calendar_support`
- `20240823_google_calendar_auth_rename`
- `20250201_event_responses_restructure`
- `20250201_optimize_event_indexes`
- `20250417_responses_collection`
- `20250420_num_responses`
- `20260730_list_item_order`

For what any individual script did, read its `main.go` — the code is the record.

`20260730_list_item_order` is the one exception to "do not re-run": it is
explicitly idempotent and takes a `-dry-run` flag, because it had to be
rehearsed against a local database before going near production. Re-running it
is a no-op, not a corruption. Its header says why.
