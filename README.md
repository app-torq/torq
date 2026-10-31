# torq

Self-hosted maintenance logbook and DIY repair tracker for people who work on their own cars.

- **Service Logbook**: routine interval maintenance (oil, fluids, filters, plugs, brakes, timing). One entry can cover several items, each with brand, spec, quantity, part numbers and cost. Logging a service resets the linked reminder automatically.
- **Self-Repair Projects**: troubleshooting, repairs, overhauls and modifications. Each project has a workflow (`Planned → In Progress → Waiting on Parts → Completed`), symptoms with OBD2 fault codes, root cause, procedure notes, critical torque specs, special tools, a parts bill of materials with per-part status, and photos, manuals and invoices.
- **Reminders**: by km, by time, or whichever comes first, with configurable "due soon" lead thresholds. Any odometer update re-evaluates them immediately.
- **Quick-reference spec sheet** per vehicle: oil and fluid specs and capacities, lug torque, tire pressures, and custom entries with images.
- **Webhook notifications** only (no email): Discord, ntfy, Gotify, Home Assistant and Generic JSON, with custom headers, a test button, a delivery log and retries.
- **Reports**: spending split by routine service vs. repairs and by parts vs. fluids, cost per km, a combined timeline, and a printable history with JSON/CSV export (for archiving or selling a car).
- English and German. Metric units only (km, L/mL, Nm, bar). One dark theme.

Out of scope by design: fuel logs and consumption, insurance/registration/inspection tracking, imperial units, light themes, email.

## Quick start (Docker)

```bash
docker compose up -d        # uses docker-compose.yml in this repository
# open http://localhost:8080 and create the administrator account
```

Or with plain Docker:

```bash
docker build -t torq .
docker run -d --name torq -p 8080:8080 -v torq-data:/data -e TZ=Europe/Berlin torq
```

The image is a ~25 MB static binary on a distroless base, runs as non-root, and needs nothing but the `/data` volume. It uses about 30 MB of RAM.

## Configuration

All settings are optional environment variables.

| Variable | Default | Purpose |
|---|---|---|
| `TORQ_DATA_DIR` | `/data` | Database, attachments and automatic backups |
| `TORQ_LISTEN_ADDR` | `:8080` | HTTP listen address |
| `TORQ_BASE_URL` | *(empty)* | Public URL, e.g. `https://torq.example.com`. Enables links in notifications and, when https, the `Secure` cookie flag |
| `TORQ_COOKIE_SECURE` | `auto` | `true`/`false` to force the `Secure` cookie flag |
| `TORQ_TRUSTED_PROXIES` | *(empty)* | Comma-separated CIDRs whose `X-Forwarded-For` header is trusted (for login throttling behind a reverse proxy) |
| `TORQ_MAX_UPLOAD_MB` | `25` | Per-file upload limit (JPEG, PNG, WebP, HEIC, PDF) |
| `TORQ_LOG_LEVEL` / `TORQ_LOG_FORMAT` | `info` / `json` | Logging (`debug`/`info`/`warn`/`error`; `json`/`text`) |
| `TZ` | `UTC` | Time zone for reminder dates and digest schedules |

## Data and backups

Everything lives in the data directory:

```
/data/torq.db (+ -wal, -shm)   SQLite database (WAL mode)
/data/attachments/             uploaded photos and PDFs
/data/backups/                 automatic copies taken before each schema upgrade (newest 3 kept)
```

To back up, copy the directory. For a consistent copy while torq is running, stop the container first or use `sqlite3 torq.db ".backup backup.db"`. Schema migrations only move forward and run automatically at startup.

## Behind a reverse proxy

- Set `TORQ_BASE_URL` to the public https URL. Session cookies then get `Secure` and notifications link back to torq.
- Set `TORQ_TRUSTED_PROXIES` to your proxy's address range so login throttling sees real client IPs.
- torq does not terminate TLS itself.

## Notifications

Add endpoints under **Settings → Webhooks** (administrators only). Each endpoint sends in its owner's language and formats.

| Preset | URL to enter | Notes |
|---|---|---|
| Discord | Channel webhook URL | Rich embeds, colored by severity |
| ntfy | Topic URL, e.g. `https://ntfy.example/torq` | Add `Authorization: Bearer …` for protected topics |
| Gotify | `https://gotify.example/message` | Add the app token as header `X-Gotify-Key` |
| Home Assistant | Webhook trigger URL `…/api/webhook/<id>` | Use `trigger.json.title`, `trigger.json.reminder.state`, … in automations |
| Generic JSON | Any URL | `torq.notification/v1` schema; see `specs/001-torq-core-platform/contracts/webhook-payloads.md` |

Header values are write-only: the API never returns them. Failed deliveries are retried after 30 s, 2 min and 10 min. Weekly and monthly digests are scheduled under **Settings → Instance**.

## Development

```bash
make setup     # go mod download + npm ci
make dev       # Go API on :8080 (data in ./.data) + Vite on :5173 (proxies /api)
make check     # go vet, golangci-lint, sqlc diff, tsc, eslint, i18n key and error-code check
make test      # go test + vitest
make e2e       # builds bin/torq and runs the Playwright suite against it
make build     # bin/torq with the SPA embedded
make docker    # production image
```

Requirements: Go 1.26+, Node.js 22, sqlc 1.27+ (only after editing `internal/store/queries`), golangci-lint v2.

Sample data for local testing: `TORQ_ENABLE_SEED=true TORQ_DATA_DIR=./.data ./bin/torq seed-demo` creates the user `demo` / `torq-demo-password` and one vehicle with a year of history. Add `--vehicles 10 --years 5` for bulk data.

Design documents (spec, plan, data model, API contract) live in `specs/001-torq-core-platform/`. Project rules are in `.specify/memory/constitution.md`.
