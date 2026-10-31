# Implementation Plan: torq Core Platform

**Branch**: `001-torq-core-platform` | **Date**: 2026-10-30 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/001-torq-core-platform/spec.md`

## Summary

torq is a self-hosted logbook for people who maintain their own cars. It covers vehicles with a quick-reference
spec sheet, a **Service Logbook** for interval maintenance that resets linked reminders, a separate
**Self-Repair Log** with diagnostics and a parts BOM, a km/time/composite **reminders engine**, outbound
**webhook notifications** (Discord, ntfy, Gotify, Home Assistant, Generic JSON), cost reports with history
export, and a dark-only, metric-only UI in English and German.

The approach is one Go binary that serves a JSON API and an embedded React SPA, stores data in SQLite
(WAL) on a single `/data` volume, and runs an in-process scheduler plus a transactional webhook outbox.
Reminder states are recomputed in the same transaction as every odometer or service write, so screens and
notifications never fall behind.

## Technical Context

**Language/Version**: Go 1.26+ (backend; current dependency versions require it), TypeScript 5.x strict (frontend)

**Primary Dependencies**:
- Backend: stdlib `net/http` + `log/slog`, `modernc.org/sqlite`, `pressly/goose/v3`, `sqlc` (codegen only),
  `golang.org/x/crypto/argon2`, `golang.org/x/text` (locale number formatting for webhook text)
- Frontend: React 19, Vite, Tailwind CSS v4, `react-router`, `@tanstack/react-query`, `i18next` +
  `react-i18next`, `lucide-react`
- Dependency budget and justification: research R14

**Storage**: SQLite 3 on `/data/torq.db` (WAL, `foreign_keys=ON`, `busy_timeout=5000`, one writer
connection plus a reader pool). Attachments are files under `/data/attachments/`. Pre-migration backups go
to `/data/backups/`.

**Testing**: Go `testing` + `httptest` against temp-file SQLite. Golden files for webhook presets. Vitest +
Testing Library. Playwright end-to-end tests against the built container (research R21).

**Target Platform**: Linux container (`linux/amd64`, `linux/arm64`), distroless static base. Evergreen
desktop and mobile browsers.

**Project Type**: Web application (Go API + embedded SPA, one deployable)

**Performance Goals**: Pages load in < 2 s on single-board-computer hardware with 10 vehicles / 5 years of
data (SC-010). History export of ≤ 500 records in < 10 s (SC-008). API p95 < 100 ms at household load.

**Constraints**: < 100 MB RSS, image < 30 MB, no network access needed except outbound webhooks, no
CGO, single `/data` volume, forward-only migrations.

**Scale/Scope**: ≤ 5 users, ≤ 20 vehicles, ~10k records, about 25 screens, 2 locales

All Technical Context unknowns are resolved in [research.md](./research.md). None are marked NEEDS
CLARIFICATION.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design (see the end of this section).*

| Principle / Non-Goal | Status | How the design complies |
|---|---|---|
| **I. Built for DIY Mechanics** | ✅ | Part numbers (OEM + aftermarket + brand), torque specs (Nm, with angle notes), fluid specs and capacities, special tools, fault codes, and photos are all first-class data. Cost reports serve repair documentation and resale history (spec Assumptions). Every user story names its workbench workflow |
| **II. Service Logbook ≠ Self-Repair Log** | ✅ | Separate tables (`service_logs`/`service_log_items` vs `repair_logs`/`repair_parts`/…) with their own lifecycles. No shared event table or type field. Only repairs have a status workflow, and only service items drive reminder baselines. Shared pieces (`attachments`, `odometer_readings`) are linked through separate explicit FKs. UI: separate navigation areas. The timeline is a read-only `UNION ALL` view (data-model §8) |
| **III. Self-Hosted First** | ✅ | One distroless container, embedded SPA, embedded SQLite in WAL mode, single `/data` volume, environment-variable configuration with defaults (R20), goose forward-only migrations applied at startup with an automatic pre-migration backup (R3). No CDN: fonts and icons are bundled. CI starts the image with only `/data` mounted |
| **IV. Isolated Authentication Boundary** | ✅ | `users.id` is the only user key domain tables reference. Credentials live in `user_identities (provider, subject, issuer, password_hash)`. argon2id hashing (R5). CSPRNG session tokens stored as SHA-256 hashes, revocable, with sliding expiry (R6). Business code depends only on `auth.UserFrom(ctx)`. The provider interface is the one sanctioned abstraction (R7) |
| **V. Localization Ready** | ✅ | i18next for all UI text, with an ESLint no-literal-string rule and a CI key-parity check. The API returns error **codes**, never prose. Webhook text comes from embedded Go catalogs with English fallback. Numbers, currency and dates are formatted with `Intl` and `x/text` formatters, not string building (R16). No unit conversion anywhere |
| **VI. Simplicity & Maintainability** | ✅ | stdlib router, sqlc-generated queries called directly from domain functions (no repositories, no DI framework), strict `tsc` and Go linters, small dependency list with a justification for each (R14). A request flows handler → domain function → `store.Queries` → SQLite |
| **No fuel tracking** | ✅ | No fuel entities or consumption metrics. Cost per km covers maintenance and repair spending only |
| **No compliance tracking** | ✅ | No insurance, registration, inspection or tax fields. Standalone reminders are for mechanical checks. The UI copy does not suggest using them for compliance |
| **Metric only** | ✅ | km / L / mL / Nm / bar are the only units, both stored and shown. No unit settings |
| **Single dark theme** | ✅ (interpretation noted) | Hardcoded zinc/slate dark classes, no `dark:` variants, no toggle. **Note**: the printable Vehicle History Report uses `print:` styles that come out black-on-white **on paper only**. This is a property of the print medium, not a user-selectable theme, and the screen always stays dark (R15) |
| **Webhooks only** | ✅ | No SMTP or email. Account recovery is done by an admin (spec Assumptions) |

**Gate result: PASS.** No violations, so Complexity Tracking stays empty.

**Post-design re-check (after Phase 1)**: data-model.md keeps the two logbooks in separate tables, and
`attachments` uses one explicit FK per owner instead of a polymorphic `owner_type` (II). `user_identities`
has `issuer` and `subject` ready for OIDC (IV). The contracts return no human-readable strings (V).
Deviations from the `/speckit-plan` input are listed under "Decisions that refine the input" below. None of
them conflict with the constitution. **PASS.**

## Decisions That Refine the Plan Input

| Input proposal | Plan decision | Why (research ref) |
|---|---|---|
| `net/http` or `chi` | stdlib `ServeMux` | Go 1.22+ patterns are enough. Constitution VI (R1) |
| `modernc` or `mattn` driver | `modernc.org/sqlite` | No CGO, static arm64 builds, distroless image (R2) |
| `golang-migrate` or `goose` | goose, forward-only, with auto-backup | Works directly on `*sql.DB` with embedded files (R3) |
| "Signed" session cookies | Opaque random token, stored hashed | Equally unforgeable and revocable, with no signing key to manage (R6) |
| `users` with `locale, currency, date_format` | Adds `role`, `number_format`, `currency_code` + optional `currency_symbol`, `disabled_at`. `user_identities` gains `issuer` | FR-007, FR-052, OIDC readiness |
| JSON spec columns on `vehicles` | 1:1 `vehicle_specs` typed table + `vehicle_spec_entries` | Typed sqlc fields and CHECKs. Custom entries can have images (data-model §2) |
| `service_categories` with name + default intervals | `builtin_key` (translated) or `custom_name`, plus `is_fluid` and `archived_at` | i18n for built-in names, Parts vs Fluids default, archive instead of delete |
| `repair_logs.diagnosis`, `torque_specs_json` | `root_cause`, `special_tools`, child tables `repair_fault_codes` and `repair_torque_specs` | Searchable fault codes (FR-025) and typed Nm values |
| Attachments "polymorphic or join-table" | One table, an explicit nullable FK per owner, exactly-one CHECK | Real FK cascades and Constitution II's explicit references |
| `reminders` with `interval_months` and a non-null category | Adds `interval_days`, nullable category + `title` (standalone checks), start baseline, `state_trigger`, `paused_at`. States `upcoming/due_soon/overdue` (instead of `ok`) | FR-027 to FR-033 |
| "Secret header" on webhooks | `webhook_headers` child table with multiple headers, values write-only | FR-036, FR-042 (R13) |
| Daily ticker | 15-minute ticker + run at startup + synchronous evaluation in write transactions | Digest hour accuracy, catching up after downtime (R9, R11) |
| Webhook worker | Transactional outbox in `webhook_deliveries`. Backoff +30 s/+2 m/+10 m. Non-retryable 4xx fail immediately | Survives restarts. Exactly one enqueue per transition (R10) |
| Kanban `Planned → In Progress → Completed` | Four columns, including `Waiting on Parts`. Cards move with a status menu, no drag-and-drop library | Spec workflow states. Dependency budget (R14) |
| Added (not in input) | `odometer_readings` table | Needed for cost per km (FR-045) and odometer history (FR-011) |
| Added (not in input) | Admin user management, instance digest schedule, `/healthz`, CSRF header | FR-007, FR-039, Constitution III health checks |

## Architecture Overview

```text
                ┌──────────────────────────── torq (single Go binary) ───────────────────────────┐
 Browser ──────▶│ net/http mux ─ middleware (recover → log → auth/session → csrf) ─┬─ /api/v1/*   │
 (React SPA)    │                                                                    └─ /* embed.FS │
                │ handlers (internal/httpapi) ──▶ domain packages ──▶ store (sqlc) ──▶ SQLite WAL  │
                │                                   │  vehicles · servicelog · repairs · reminders  │
                │                                   │  reports · attachments · auth                 │
                │                                   └─ in same tx: reminders.EvaluateVehicle()      │
                │                                          └─ notify.Enqueue() → webhook_deliveries │
                │ scheduler (15 min tick) ── reminders.EvaluateAll · notify.EnqueueDigests · prune  │
                │ notify.Worker ── claims pending deliveries ── presets ──▶ HTTP POST ──────────────┼──▶ Discord / ntfy /
                └──────────────────────────────────────────────────────────────────────────────────┘   Gotify / HA / any
                                     /data: torq.db (+wal) · attachments/ · backups/
```

**Write path example (log a service)**: `POST /api/v1/service-logs` → `httpapi.createServiceLog` decodes
and validates → `servicelog.Create(ctx, db, input)` opens a `BEGIN IMMEDIATE` tx → inserts the log and its
items → `vehicles.RecordReading(tx, …)` applies the odometer rules → `reminders.EvaluateVehicle(tx,
vehicleID, today)` recomputes baselines and states → `notify.EnqueueTransitions(tx, transitions)` → commit →
`notify.Worker.Wake()` → the response includes the re-evaluated reminders.

## Project Structure

### Documentation (this feature)

```text
specs/001-torq-core-platform/
├── plan.md              # This file
├── research.md          # Phase 0: decisions R1–R22
├── data-model.md        # Phase 1: tables, invariants, state machines
├── quickstart.md        # Phase 1: run + validation scenarios V1–V9
├── contracts/
│   ├── rest-api.md      # Phase 1: /api/v1 contract
│   └── webhook-payloads.md
├── checklists/requirements.md
└── tasks.md             # Phase 2 (/speckit-tasks; not created here)
```

### Source Code (repository root)

```text
cmd/torq/
└── main.go                    # config, open DB, migrate, start scheduler + worker + HTTP; subcommands: healthcheck, seed-demo (dev)

internal/
├── config/                    # env parsing with defaults (R20)
├── db/                        # Open() writer/reader handles + pragmas; Migrate() with goose + pre-migration backup
│   └── migrations/            # 0001_init.sql … (embedded, forward-only)
├── store/                     # sqlc output (committed)
│   └── queries/               # users.sql, vehicles.sql, service_logs.sql, repairs.sql, reminders.sql, webhooks.sql, reports.sql, …
├── auth/                      # User, UserFrom(ctx), RequireUser/RequireAdmin, sessions, throttle, Provider interfaces
│   └── local/                 # argon2id password provider
├── httpapi/                   # router.go, middleware.go, errors.go, one file per area (vehicles.go, service_logs.go, repairs.go, reminders.go, webhooks.go, reports.go, users.go, attachments.go, setup.go)
├── vehicles/                  # profile, spec sheet, odometer rules (RecordReading)
├── servicelog/                # service sessions + items, categories
├── repairs/                   # projects, status transitions, BOM, fault codes, torque specs
├── reminders/
│   ├── engine/                # pure NextDue/Evaluate + month arithmetic (exhaustively unit-tested)
│   └── evaluate.go            # EvaluateVehicle / EvaluateAll (DB-backed, returns transitions)
├── notify/
│   ├── outbox.go              # EnqueueTransitions, EnqueueDigests, test send
│   ├── worker.go              # delivery loop, retries, retention
│   └── presets/               # discord.go, ntfy.go, gotify.go, homeassistant.go, generic.go + testdata/ golden files
├── reports/                   # cost aggregation, timeline, JSON/CSV history export
├── attachments/               # file store, sniffing, serving, orphan sweep
├── scheduler/                 # 15-min ticker orchestration
└── i18n/                      # embedded en.json/de.json for webhook text + x/text number formatting

web/                           # Vite + React SPA (package "web" also hosts embed.go)
├── embed.go                   # //go:embed all:dist
├── index.html
├── src/
│   ├── main.tsx, app/         # router, QueryClient, AuthGate, Layout (sidebar + vehicle switcher), QuickActions (⌘K)
│   ├── lib/                   # api.ts (fetch wrapper: CSRF header, error codes), format.ts (Intl), units.ts, queryKeys.ts
│   ├── i18n/                  # setup + locales/{en,de}/{common,auth,vehicles,specs,service,repairs,reminders,reports,settings,webhooks,errors}.json
│   ├── components/            # Button, Field, Dialog, Table, Badge (state colors), FileDrop, EmptyState, …
│   └── features/
│       ├── auth/              # Setup, Login, ChangePassword
│       ├── dashboard/         # attention list, odometer quick-update, open repairs
│       ├── vehicles/          # list, profile form, spec sheet pinboard
│       ├── service-log/       # list/filter, multi-item entry form with spec hints
│       ├── repairs/           # board (4 columns) / list toggle, project detail, BOM table
│       ├── reminders/         # list, form (km/time/composite), mark done
│       ├── reports/           # cost report, timeline, history print route + export buttons
│       └── settings/          # preferences, users (admin), webhooks (admin) + delivery log, instance
├── e2e/                       # Playwright scenarios from quickstart.md
└── scripts/check-i18n.ts

Dockerfile                     # node build → go build (CGO_ENABLED=0) → distroless static
Makefile                       # setup, dev, generate, check, test, e2e, build, docker
sqlc.yaml
go.mod
```

**Structure Decision**: One repository and one deployable. The Go module is at the root. The SPA lives
in `web/`, and its build output (`web/dist`) is embedded by the Go package `web`, because `go:embed` can't
reach parent directories, so the embed file sits next to `dist`. Backend code is organized by domain
(`vehicles`, `servicelog`, `repairs`, …). The service logbook and repair log are separate packages that do
not import each other (Constitution II). `httpapi` is the only package that knows about HTTP.

## Implementation Phasing (input for /speckit-tasks)

Each phase can be delivered and demonstrated on its own, following the spec's story priorities.

1. **Foundation**: repository scaffolding, config, DB open + migrations, sqlc setup, HTTP skeleton +
   middleware + error codes, SPA shell (dark layout, i18n, format lib, API client), Dockerfile, CI gates.
2. **US1 Auth & garage (P1)**: setup, login/logout, sessions, throttle, password change, users admin,
   vehicles CRUD, avatar upload, odometer quick-update + readings, dashboard skeleton, ⌘K palette (odometer
   action).
3. **US2 + US3 Service logbook & reminders (P1)**: categories, multi-item service form with attachments,
   reminder engine + evaluation in the write transaction, reminders UI, dashboard attention list, scheduler
   tick.
4. **US4 Self-repair (P2)**: projects, status transitions with completion side effects, BOM, fault
   codes/torque specs/tools, board/list views, search.
5. **US5 Spec sheet (P2)**: typed specs, custom entries with images and reorder, hints in the service form.
6. **US6 Webhooks (P2)**: endpoint admin, header masking, presets + golden tests, outbox + worker + retries,
   test send, delivery log, digests.
7. **US7 Reports (P3)**: cost aggregation, cost per km, timeline, JSON/CSV export, print route.
8. **US8 Localization polish (P3)**: complete `de` catalogs, preference UI, i18n CI gate enforced as
   blocking.
9. **Hardening**: e2e suite, footprint check (V9), orphan sweep, backup rotation, docs (README: run,
   volumes, env vars, reverse-proxy notes).

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| sqlc's SQLite engine has gaps (some functions and `RETURNING` edge cases) | Keep queries simple, put reporting `UNION ALL` queries in sqlc where they parse, and fall back to hand-written `database/sql` in `reports/` only where needed, with a comment explaining why |
| Webhook preset formats drift (e.g. Discord embed limits) | Golden tests, truncation to the documented limits (title 256, description 4096, 25 fields), and V6 manual checks before each release |
| Odometer typos cause floods of false notifications | Confirmation for decreases and large jumps, one notification per transition, and no notification on downgrades |
| Low-memory devices | argon2 parameters sized for a single-board computer (R5), no thumbnails, streamed uploads and exports |
| HEIC photos from iPhones can't be previewed | Stored and downloadable. Preview shows a file tile. Conversion is deferred |

## Complexity Tracking

No constitution violations, so nothing to justify.
