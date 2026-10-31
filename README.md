# torq

Self-hosted maintenance logbook and DIY repair tracker for people who work on their own cars.

- **Service Logbook**: routine interval maintenance (oil, fluids, filters, plugs, brakes, timing). One entry can cover several items, each with brand, spec, quantity, part numbers and cost. Logging a service resets the linked reminder automatically.
- **Self-Repair Projects**: troubleshooting, repairs, overhauls and modifications. Each project has a workflow (`Planned → In Progress → Waiting on Parts → Completed`), symptoms with OBD2 fault codes, root cause, procedure notes, critical torque specs, special tools, a parts bill of materials with per-part status, and photos, manuals and invoices.
- **Reminders**: by km, by time, or whichever comes first, with configurable "due soon" lead thresholds. Any odometer update re-evaluates them immediately.
- **Offline vehicle catalog**: type the HSN/TSN from the German registration certificate (Fahrzeugschein fields 2.1/2.2) to fill make and model, or pick make → model → chassis code (W203, E46, Golf IV (1J), …) → engine from searchable lists that fill power (kW), displacement (cm³) and fuel type. Model names follow your language (C-Class / C-Klasse). Any field also accepts your own text. Lookups never leave your server and need no API keys; admins can refresh the catalog from its sources with one click.
- **Quick-reference spec sheet** per vehicle: oil and fluid specs and capacities, lug torque, tire pressures, and custom entries with images.
- **Webhook notifications** only (no email): Discord, ntfy, Gotify, Home Assistant and Generic JSON, with custom headers, a test button, a delivery log and retries.
- **Reports**: spending split by routine service vs. repairs and by parts vs. fluids, cost per km, a combined timeline, and a printable history with JSON/CSV export (for archiving or selling a car).
- English and German. Metric units only (km, L/mL, Nm, bar); power shows kW or PS first, per user. One dark theme.

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
| `TORQ_CATALOG_ONLINE_UPDATES` | `true` | Lets admins refresh the vehicle catalog from KBA and GitHub (*About & data sources* → *Update now*). `false` keeps torq fully offline |
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

## Vehicle data & licenses

The vehicle catalog ships inside torq and is imported into the local database on first start after an upgrade. Lookups run entirely offline. Admins can also refresh it under **Settings → About & data sources → Update now**: torq then downloads the current FZ 6 workbook from the KBA and the latest vehicle-makes-models data from GitHub, rebuilds the catalog and replaces it in one transaction. An online catalog is kept across restarts until a release bundles newer data; set `TORQ_CATALOG_ONLINE_UPDATES=false` to disable the button. The bundled data comes from:

- **Kraftfahrt-Bundesamt (KBA), FZ 6** — "Bestand an Kraftfahrzeugen und Kraftfahrzeuganhängern nach Herstellern und Typen", stand 1 January 2024, [kba.de](https://www.kba.de/DE/Statistik/Fahrzeuge/Bestand/ListeHerstellerTypSchluesselnummern/typschluesselnummern_node.html).
  © Kraftfahrt-Bundesamt, Flensburg. Datenquelle: Kraftfahrt-Bundesamt, FZ 6, 2026-10-31; [Datenlizenz Deutschland – Namensnennung – Version 2.0](https://www.govdata.de/dl-de/by-2-0) (dl-de/by-2-0); Daten geändert (data modified by torq).
- **vehicle-makes-models** — [github.com/gor3a/vehicle-makes-models](https://github.com/gor3a/vehicle-makes-models), `data/json/all.json` at commit `ee99776470a45214bf524cdfe22e5dbf4bd67f93`, compiled from publicly available information on [autoevolution.com](https://www.autoevolution.com). Licensed under the [Open Database License (ODbL) 1.0](https://opendatacommons.org/licenses/odbl/1-0/); changes were made.

The combined catalog database (`internal/catalog/data/catalog_seed.json.gz`) is a derived database and is distributed under the ODbL 1.0 ([full text](internal/catalog/data/LICENSE-ODbL.txt)). Sources, retrieval dates and every transformation are documented in [`internal/catalog/data/NOTICE.md`](internal/catalog/data/NOTICE.md). The torq program code is licensed separately.

The KBA's SV 4.2 directory (with kW, cm³ and fuel per HSN/TSN) is **not** bundled: the KBA publishes it as "all rights reserved". Because of this, an HSN/TSN lookup fills make and model; power, displacement and fuel come from the engine list or manual entry. The same attributions are shown in the app under **Settings → About & data sources**.

Maintainers refresh the bundled catalog with `make catalog` (see the Makefile). Chassis codes come from the hand-maintained table `internal/catalog/catalogsync/chassis.txt`, because the sources only carry production years; corrections are welcome.

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

Design documents (spec, plan, data model, API contract) live in `specs/001-torq-core-platform/` and `specs/002-vehicle-catalog-lookup/`. Project rules are in `.specify/memory/constitution.md`.
