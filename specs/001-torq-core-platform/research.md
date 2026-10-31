# Research: torq Core Platform

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Date**: 2026-10-30

Each decision resolves a technical unknown or refines the stack proposed in the `/speckit-plan` input
against the constitution (Principles I–VI, Hard Non-Goals).

---

## R1. Go toolchain and HTTP routing

- **Decision**: Go 1.23+ (`go.mod` declares `go 1.23`; CI builds with the current stable toolchain). Routing
  uses the standard library `net/http.ServeMux` with method + path-wildcard patterns
  (`GET /api/v1/vehicles/{id}`). Logging uses `log/slog` (JSON in production, text in development).
- **Rationale**: Since Go 1.22 the standard mux covers everything torq needs (method matching, path
  parameters). Constitution VI prefers standard-library primitives, and middleware is just
  `func(http.Handler) http.Handler`.
- **Alternatives considered**: `chi` is a good router, but adds a dependency for features the stdlib already
  provides. Echo/Gin/Fiber are heavier frameworks with their own context types, which makes requests harder
  to trace to the database.

## R2. SQLite driver and connection handling

- **Decision**: `modernc.org/sqlite` (pure Go, no CGO). Two `*sql.DB` handles on the same file: a **writer**
  limited to one connection (`SetMaxOpenConns(1)`) and a **reader** pool (e.g. 4 connections). Pragmas set via
  DSN on every connection: `journal_mode=WAL`, `foreign_keys=ON`, `busy_timeout=5000`, `synchronous=NORMAL`.
  Write transactions use `BEGIN IMMEDIATE`.
- **Rationale**: `CGO_ENABLED=0` gives a static binary. That allows a distroless/static base image and simple
  `linux/amd64` + `linux/arm64` builds for home servers and single-board computers. A single writer avoids
  `SQLITE_BUSY` upgrade deadlocks under WAL. At torq's scale (≤ 20 vehicles, a few thousand records), the
  modernc speed penalty doesn't matter.
- **Alternatives considered**: `mattn/go-sqlite3` is faster but needs CGO, which complicates cross-compiling
  and the base image. `ncruces/go-sqlite3` (WASM) is newer and less established with sqlc/goose.

## R3. Migrations

- **Decision**: `pressly/goose/v3` with SQL migration files embedded via `embed.FS`, applied automatically at
  startup before the HTTP server starts listening. Migrations are **forward-only**: files contain only an
  `-- +goose Up` section and the down command is never run. A pre-migration backup copy of the database
  (`/data/backups/torq-<version>-<timestamp>.db`, made with `VACUUM INTO`) is taken whenever there are
  pending migrations; the last 3 are kept.
- **Rationale**: Goose works directly on a `*sql.DB` with the modernc driver, supports embedded files, and
  has a small API. Constitution III requires automatically applied, forward-only migrations. The automatic
  backup gives self-hosters a way back without supporting down migrations.
- **Alternatives considered**: `golang-migrate` needs a driver-specific database package and has a larger
  dependency tree. A hand-rolled runner would be small, but goose is well maintained and saves edge-case
  work (version table, locking).

## R4. Query layer

- **Decision**: `sqlc` (engine `sqlite`, `database/sql` output) generates typed query code into
  `internal/store`. The generated code is committed, and CI fails if `sqlc generate` produces a diff. Schema
  input = the goose migration directory. Queries live in `internal/store/queries/*.sql`, one file per area.
  Dynamic filters (optional vehicle/category/date/search filters) use the
  `(sqlc.narg(x) IS NULL OR col = sqlc.narg(x))` pattern rather than string-built SQL.
- **Rationale**: Type-safe SQL without an ORM keeps the path from request to database readable
  (Constitution VI). Domain code calls `store.Queries` methods directly; there are no repository interfaces.
- **Alternatives considered**: GORM/ent (heavy abstraction, contrary to VI). Hand-written `database/sql`
  (more boilerplate and no compile-time column checks).

## R5. Password hashing

- **Decision**: `golang.org/x/crypto/argon2` (`IDKey`), stored as a PHC string
  (`$argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>`). Parameters: 19 MiB memory, 2 iterations, parallelism 1,
  16-byte salt, 32-byte key. Hashes with outdated parameters are re-hashed on successful login.
- **Rationale**: Constitution IV mandates argon2id. These parameters are OWASP's minimum recommendation and
  fit a low-memory single-board computer (each login briefly uses about 19 MiB). Login attempts are
  throttled, which also limits memory spikes from login bursts.
- **Alternatives considered**: Higher memory costs (64 MiB+) risk memory pressure on 512 MB–1 GB devices.
  bcrypt/scrypt are not allowed by the constitution.

## R6. Sessions and CSRF

- **Decision**: An opaque 32-byte token from `crypto/rand`, sent base64url-encoded in the `torq_session`
  cookie (`HttpOnly`, `SameSite=Lax`, `Path=/`, `Secure` when `TORQ_BASE_URL` is https or
  `TORQ_COOKIE_SECURE=true`). The `sessions` table stores **SHA-256(token)**, never the token. Sliding
  expiry: 30 days of inactivity (`last_seen_at` refreshed at most hourly). Logout deletes the row. A password
  change deletes all of the user's other sessions. Expired rows are pruned by the scheduler.
  CSRF: all non-GET `/api/v1/*` requests must carry `X-Torq-Request: 1`, and the `Origin` header (when
  present) must match the request host or `TORQ_BASE_URL`. torq never sends CORS allow headers, so a
  cross-site page cannot set the custom header.
- **Rationale**: The plan input asked for *signed* cookies. A random token whose hash is looked up in the
  database already can't be forged or guessed. Adding an HMAC signature would only add a secret key to
  manage, without extra security. Storing hashes satisfies Constitution IV (revocable, non-reversible,
  expiring).
- **Alternatives considered**: Signed stateless cookies/JWT (can't be revoked without a deny-list). CSRF
  synchronizer tokens (more state; the custom-header + SameSite approach is enough for a JSON-only API).

## R7. Authentication provider boundary (OIDC readiness)

- **Decision**: Package `internal/auth` exposes:
  - `type User struct { ID int64; Username, DisplayName string; Role Role; Prefs Preferences }` and
    `auth.UserFrom(ctx) (User, bool)` / `auth.RequireUser`/`auth.RequireAdmin` middleware. This is the
    only auth surface the rest of the app uses.
  - `type Provider interface { Kind() string }`, plus capability interfaces such as
    `PasswordAuthenticator { AuthenticatePassword(ctx, username, password string) (userID int64, err error) }`.
    v1 registers `local.Provider`. A future `oidc.Provider` would add a redirect/callback capability and
    resolve `(provider, subject)` → `user_id` through `user_identities`.
  - Session issuance (`auth.Sessions.Create(ctx, userID)`) doesn't depend on the provider.
- **Rationale**: This satisfies Constitution IV: domain tables reference `users.id` only. Credentials live in
  `user_identities` (`provider`, `subject`, nullable `password_hash`, unique `(provider, subject)`).
  Business packages never import `auth/local`.
- **Alternatives considered**: One generic `Authenticate(credentials any)` method (loses type safety, and
  OIDC is a redirect flow rather than a credential check).

## R8. Login throttling

- **Decision**: In-memory limiter keyed by `username` and by client IP. After 5 consecutive failures, each
  further attempt is delayed exponentially (1 s → max 15 min lockout window). Success resets the counter.
  The client IP comes from `RemoteAddr`, or from `X-Forwarded-For` only when the peer is in
  `TORQ_TRUSTED_PROXIES` (CIDR list).
- **Rationale**: Meets FR-006 without a table. Losing state on restart is acceptable for a home-scale app.
- **Alternatives considered**: Persisting attempts in SQLite (extra writes for little value).

## R9. Reminder engine

- **Decision**: A pure function package `internal/reminders/engine`:
  `NextDue(baseline, interval) (km *int64, date *civil.Date)` and
  `Evaluate(r Reminder, currentKm int64, today Date) Status{State, RemainingKm, RemainingDays, Trigger}`.
  - Months are added with clamping to the end of the month (Jan 31 + 1 month = Feb 28/29).
  - State = `overdue` if any limit is reached, otherwise `due_soon` if within any lead threshold, otherwise
    `upcoming`. `Trigger` records which dimension (km or time) produced the most urgent state.
  - Baseline for category-linked reminders = most recent service log item of that category for that vehicle
    (ordered by `service_date DESC, odometer_km DESC`), falling back to the manual start point. Standalone
    reminders use their last "mark done" values (or the start point).
- **Re-evaluation triggers**: (a) synchronously, inside the **same transaction** as any write that changes
  a vehicle's odometer, a service log, a reminder definition, or a vehicle's archived state. Only that
  vehicle's reminders are evaluated. (b) The scheduler tick (R11) for all active reminders, so time-based
  states advance without user activity, including once right after startup.
- **Rationale**: Pure functions are easy to test exhaustively with tables (month clamping, composite
  intervals, back-filled history). Evaluating in the same transaction makes SC-003 hold by construction.
- **Alternatives considered**: Computing state only on read (can't detect transitions for notifications).
  Evaluating only in a background job (the UI would show stale state right after writes).

## R10. Notification outbox and idempotency

- **Decision**: Each reminder stores `state` and `last_notified_state`. When an evaluation finds a
  reminder **entering** `due_soon` or `overdue` and the new state differs from `last_notified_state`, the
  same transaction inserts one `webhook_deliveries` row (status `pending`, rendered payload) per enabled
  endpoint subscribed to that event, then sets `last_notified_state` = new state. When a reminder returns to
  `upcoming` (e.g., after a service), `last_notified_state` is reset to `upcoming`, so a later re-entry
  notifies again (FR-038). Paused reminders and reminders of archived vehicles are skipped and never
  enqueue anything.
- **Delivery worker**: one goroutine. It is woken via a channel after commits that enqueued work, and also
  polls every 30 s. It claims due rows (`status='pending' AND next_attempt_at <= now`), sends them with a
  10 s timeout, and records each attempt. The retry schedule is +30 s, +2 min, +10 min (3 retries, about
  12.5 min total), then `failed`. Responses 2xx = `succeeded`. 4xx other than 408/429 fail immediately
  (configuration errors won't fix themselves). Redirects are not followed. The response excerpt is capped at
  2 KiB. The last 100 finished deliveries are kept per endpoint.
- **Rationale**: A transactional outbox gives exactly-once *enqueueing* per transition (SC-005) and survives
  restarts. Delivery is at-least-once in the rare crash-during-send case, which is acceptable for
  notifications.
- **Alternatives considered**: Sending from inside the request (blocks the user and loses messages on
  failure). An in-memory channel queue (loses pending messages on restart).

## R11. Scheduler

- **Decision**: One in-process goroutine with a 15-minute `time.Ticker`, also run once at startup. Each
  tick: (1) re-evaluate all active reminders (time-based transitions; R9/R10), (2) enqueue weekly/monthly
  digests when the configured local day and hour has passed and the endpoint's `last_*_digest_on` is older
  than the current period, (3) prune expired sessions and old delivery logs.
  The time zone comes from the container's `TZ` (Go loads tzdata via the embedded `time/tzdata` import so
  distroless works).
- **Rationale**: A 15-minute tick easily meets SC-004 (≤ 24 h) and digest timing, and costs almost nothing.
  The startup run covers downtime (the "app offline" edge case).
- **Alternatives considered**: A daily-only tick (could fire digests up to a day late and makes the
  configured hour meaningless). A cron library (an unnecessary dependency).

## R12. Webhook payload presets

- **Decision**: One formatter per preset in `internal/notify/presets`. Each takes a localized `Message`
  (title, body, URL, severity, structured fields) and returns method, headers and body:

  | Preset | Target URL the user enters | Request |
  |---|---|---|
  | Discord | Discord channel webhook URL | `POST` JSON `{username:"torq", embeds:[{title, description, url, color, fields, timestamp}]}`. Color: amber = due soon, red = overdue, zinc = digest/test |
  | ntfy | Topic URL (`https://ntfy.example/torq`) | `POST` body = plain-text message; headers `Title`, `Priority` (default/3, high/4 for overdue), `Tags` (`wrench` / `warning`), `Click` (deep link if base URL set). Avoids non-ASCII header problems by RFC 2047-encoding titles |
  | Gotify | `https://gotify.example/message` (token via custom header `X-Gotify-Key` or `?token=`) | `POST` JSON `{title, message, priority (5/8), extras:{"client::notification":{"click":{"url":…}}, "client::display":{"contentType":"text/markdown"}}}` |
  | Home Assistant | Webhook trigger URL (`/api/webhook/<id>`) | `POST` JSON `{event, title, message, vehicle, reminder, state, url, …}`, i.e. the Generic JSON body, which HA automations read via `trigger.json` |
  | Generic JSON | Any URL | `POST` JSON per [contracts/webhook-payloads.md](./contracts/webhook-payloads.md), versioned by `schema: "torq.notification/v1"` |

- **Rationale**: These are the native formats each target displays without extra configuration (SC-006). The
  header-based ntfy form needs no topic duplication in the body.
- **Alternatives considered**: ntfy JSON publishing to the server root (the user would have to enter server
  and topic separately). Home Assistant REST `notify` services (needs a long-lived token and a chosen notify
  target; the webhook trigger is simpler and token-free).

## R13. Webhook secrets and target restrictions

- **Decision**: Custom headers are stored in a `webhook_headers` child table. Values are stored as-is,
  because they must be replayed, and are **never returned by the API**: responses contain
  `{name, value_set: true}`. PATCH replaces a value only when a new one is supplied (FR-042). Only
  administrators can manage endpoints. Allowed schemes are `http`/`https`. Private/LAN addresses are
  allowed on purpose (self-hosted Home Assistant/Gotify/ntfy).
- **Rationale**: Encrypting at rest would need a key stored in the same `/data` volume or in the
  environment. That adds key management for little real protection, since the database and attachments
  already sit in the same trust boundary.
- **Alternatives considered**: Encrypting with `TORQ_SECRET_KEY` (deferred; can be added later with a
  migration if users ask for it).

## R14. Frontend stack and dependency budget

- **Decision**: React 19 + TypeScript (`strict`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`)
  + Vite. Runtime dependencies, each justified:
  - `react-router` (data router) for nested routes such as `/vehicles/:id/...` and the printable report
    route.
  - `@tanstack/react-query` for server-state caching and **invalidation**. After an odometer, service or
    repair write, the client invalidates `vehicle`, `reminders` and `dashboard` queries, so every screen
    shows re-evaluated states without a manual refresh (SC-003).
  - `i18next` + `react-i18next` for localization (Constitution V).
  - `lucide-react` for icons.
  - `tailwindcss` v4 via `@tailwindcss/vite` (build-time only).
  - No component kit, form library, chart library or drag-and-drop library. Forms use controlled
    components and a small `useForm` helper. Report breakdowns are tables plus CSS bar meters. The repair
    Kanban moves cards with a status menu and keyboard shortcuts (drag-and-drop can come later). The
    `Ctrl/⌘ K` quick-action palette is a ~150-line component.
- **Rationale**: Constitution VI requires every new dependency to be justified. The excluded categories are
  where frontends usually pick up most of their weight.
- **Alternatives considered**: Plain `fetch` + context for state (cache invalidation across screens gets
  hand-rolled and buggy). Redux/Zustand (not needed when server state is in Query). Recharts/Chart.js (too
  heavy for two bar breakdowns).

## R15. Dark-only styling and the printable report

- **Decision**: `<html class="… bg-zinc-950 text-zinc-100" style="color-scheme: dark">`. No `dark:` variants
  and no theme state. Tailwind's default zinc/slate palette, with accent colors only for state (amber = due
  soon, red = overdue, emerald = completed/installed, sky = links/focus). Focus rings are always visible
  (`focus-visible:ring-2 ring-sky-400`). Text contrast is at least WCAG AA (zinc-100/zinc-300 on
  zinc-950/900).
  The Vehicle History Report route uses Tailwind `print:` variants that render black on white **only on
  paper**. On screen it is dark like every other page.
- **Rationale**: Hard Non-Goal "Single dark theme" forbids light mode and theme toggles. Print styles are a
  property of the paper medium, not a user-selectable theme, and printing light-on-black wastes toner and is
  hard to read. This interpretation is recorded in the Constitution Check.
- **Alternatives considered**: Printing in dark colors (poor print results; browsers often drop background
  colors, leaving light-gray text on white).

## R16. Localization and formatting

- **Decision (frontend)**: i18next with bundled resources (no runtime HTTP backend, so it works offline).
  Namespaces: `common`, `auth`, `vehicles`, `specs`, `service`, `repairs`, `reminders`, `reports`,
  `settings`, `webhooks`, `errors`. Locales are `en` (reference) and `de`, with `fallbackLng: 'en'`.
  `lib/format.ts` builds formatters from the user's preferences:
  - numbers: `Intl.NumberFormat(numberFormat === 'comma' ? 'de-DE' : 'en-US')`
  - currency: `Intl.NumberFormat(locale, {style:'currency', currency: currencyCode, currencyDisplay:'narrowSymbol'})`.
    A custom free-text symbol replaces the `currency` part via `formatToParts`.
  - dates: `Intl.DateTimeFormat(locale, {year:'numeric', month:'2-digit', day:'2-digit'}).formatToParts`,
    reordered for `DD.MM.YYYY` or `YYYY-MM-DD`
  - units are fixed metric suffixes from translation keys (`units.km`, `units.l`, `units.ml`, `units.nm`,
    `units.bar`)
  A CI script (`web/scripts/check-i18n.ts`) fails when `de` lacks keys that exist in `en`, or when JSX
  contains string literals outside an allowlist (ESLint `i18next/no-literal-string`).
- **Decision (backend)**: The API never returns user-facing prose. Errors are machine codes with field
  details (`{"error":{"code":"reminder.lead_exceeds_interval","fields":{…}}}`) that the frontend
  translates. The only server-rendered text is webhook payloads. These use `internal/i18n` with embedded
  `en.json`/`de.json` catalogs (English fallback) and `golang.org/x/text/message` + `language` for number
  formatting in the endpoint owner's locale.
- **Rationale**: Constitution V (no hardcoded strings, locale-aware formatters, English fallback). Machine
  error codes keep all translations in the frontend.
- **Alternatives considered**: Server-side translation of API errors (two catalogs to keep in sync for the
  same strings).

## R17. Money, quantities and measurements

- **Decision**:
  - Money: `INTEGER` minor units (cents). Two minor units are assumed for all currencies (documented
    assumption; torq never converts currencies). Line total = `quantity × unit_cost_cents` (integer
    arithmetic).
  - BOM quantity: `INTEGER ≥ 1` (pieces). Fluids bought in bulk are entered as quantity 1 with the total
    cost.
  - Fluid quantity on service items: `quantity_ml INTEGER` plus `quantity_unit ('L'|'mL')`, which records
    how the user entered it so it displays the same way.
  - Capacities on the spec sheet: `INTEGER` mL.
  - Torque (Nm) and pressure (bar): `REAL`. These are physical measurements, not money, and are entered with
    at most 1–2 decimals.
  - Odometer: `INTEGER` km.
- **Rationale**: Exact cost sums (SC-007) without floating point. Metric storage as required by the Hard
  Non-Goals.

## R18. Attachments

- **Decision**: Files are stored at `/data/attachments/<yyyy>/<mm>/<random-128-bit-hex>` with metadata in
  the `attachments` table. Upload limit: `TORQ_MAX_UPLOAD_MB` (default 25) via `http.MaxBytesReader`. Type
  allowlist: JPEG, PNG, WebP, HEIC/HEIF, PDF. The type is checked by **content sniffing**
  (`http.DetectContentType` plus an ISO-BMFF `ftyp` brand check for HEIC), not by the client-supplied
  type. Files are served only to authenticated users, with the stored MIME type, `X-Content-Type-Options:
  nosniff`, `Content-Security-Policy: sandbox`, and `Content-Disposition` (`inline` for images/PDF,
  `attachment` for HEIC, which most browsers can't display). Deleting an owner removes the rows by FK
  cascade. Files are removed after commit, and a startup sweep deletes orphaned files older than 1 hour.
  No thumbnails in v1. The UI uses `loading="lazy"` and CSS `object-fit`.
- **Rationale**: Everything lives in one data volume (Constitution III). Sniffing prevents stored-XSS via
  disguised HTML/SVG uploads.
- **Alternatives considered**: Storing blobs in SQLite (inflates the database and WAL, slows backups of the
  DB file). Thumbnail generation (needs image libraries, especially for HEIC; deferred).

## R19. Exports and the printable report

- **Decision**:
  - **JSON**: `GET /api/v1/vehicles/{id}/history?format=json` returns a single document with
    `schema: "torq.vehicle-history/v1"`, the vehicle profile, spec sheet, service logs with items, completed
    repairs with fault codes, torque specs, BOM, and attachment metadata (names, types, sizes; no file
    contents).
  - **CSV**: `…?format=csv`, a single flat RFC 4180 file, UTF-8 with BOM (so Excel opens umlauts
    correctly), one row per service item / BOM line / repair without parts. Columns:
    `record_type, date, odometer_km, record_title, category_or_part, brand, specification, quantity, unit,
    oem_part_number, aftermarket_part_number, cost, cost_type, status, notes`. Cost values are plain decimals
    (`89.50`) with the dot separator, for machine readability.
  - **Printable view**: frontend route `/vehicles/:id/history/print` using the same JSON.
  - Query flags `include_costs` and `include_notes` (both default `true`) apply to all three.
- **Rationale**: One flat CSV opens directly in spreadsheets (the common need when selling a car). JSON is
  complete and versioned so a future import is possible.
- **Alternatives considered**: A ZIP of several CSVs (more complete relations, but harder for the typical
  user). Server-side PDF generation (needs a PDF library; browser print-to-PDF is enough).

## R20. Packaging and runtime configuration

- **Decision**: A three-stage Dockerfile:
  1. `node:22-alpine` builds `web/` and outputs `web/dist`
  2. `golang:1.2x-alpine` builds with `CGO_ENABLED=0 -trimpath -ldflags "-s -w -X …version"` and embeds
     `web/dist` via `//go:embed all:dist` in package `web`
  3. `gcr.io/distroless/static-debian12:nonroot`, containing just the binary
  Multi-arch images (`linux/amd64`, `linux/arm64`) are built with buildx. `VOLUME /data`, `EXPOSE 8080`.
  The `HEALTHCHECK` runs `torq healthcheck` (a subcommand that GETs `/healthz`), because distroless has no
  curl.
  The SPA fallback serves `index.html` for unknown non-`/api` paths. Hashed assets get
  `Cache-Control: immutable`.
  Environment variables (all optional):

  | Variable | Default | Purpose |
  |---|---|---|
  | `TORQ_DATA_DIR` | `/data` | Database, attachments, backups |
  | `TORQ_LISTEN_ADDR` | `:8080` | HTTP listen address |
  | `TORQ_BASE_URL` | *(empty)* | External URL for webhook deep links and the `Secure` cookie decision |
  | `TORQ_COOKIE_SECURE` | auto | Force the `Secure` cookie flag |
  | `TORQ_TRUSTED_PROXIES` | *(empty)* | CIDRs whose `X-Forwarded-For` is trusted |
  | `TORQ_MAX_UPLOAD_MB` | `25` | Per-file upload limit |
  | `TORQ_LOG_LEVEL` / `TORQ_LOG_FORMAT` | `info` / `json` | Logging |
  | `TZ` | `UTC` | Time zone for reminder dates and digests |

- **Rationale**: Single container, single volume, no CDN, safe defaults (Constitution III). The image size
  target is under 30 MB.
- **Alternatives considered**: An Alpine runtime base (larger, and the shell isn't needed). TLS termination
  in torq (left to the user's reverse proxy).

## R21. Testing and quality gates

- **Decision**:
  - **Go**: stdlib `testing` + `net/http/httptest`. Every integration test gets a fresh temp-file SQLite
    database with migrations applied (fast, and matches production). Table-driven tests for the reminder
    engine, money/report aggregation and month arithmetic. Golden-file tests for each webhook preset (en +
    de). An HTTP contract test suite checks the routes in [contracts/rest-api.md](./contracts/rest-api.md).
    Linting: `go vet`, `staticcheck`, `golangci-lint` (errcheck, gosec, revive).
  - **Web**: `tsc --noEmit` (strict), ESLint (`typescript-eslint` strict + `eslint-plugin-i18next`),
    Vitest + Testing Library for formatters, the quick-action palette, and forms with conditional
    validation.
  - **End-to-end**: Playwright (dev dependency only) runs the [quickstart](./quickstart.md) scenarios
    against the built container in CI, which also checks Constitution III (the image starts with only
    `/data` mounted).
- **Rationale**: Covers the PR gates in the constitution (types, lint, tests, Docker start).
- **Alternatives considered**: An in-memory SQLite for tests (behaves differently from WAL on disk). Cypress
  (heavier, slower in CI).

## R22. Household data scope (FR-009)

- **Decision**: One shared garage. Domain tables have **no owner column** for access control. They carry
  `created_by` (nullable FK, `ON DELETE SET NULL`) for attribution only. Roles: `admin` (manages users,
  webhooks and instance settings) and `member` (everything else).
- **Rationale**: Matches the clarified spec and the proposed schema (`vehicles` without `user_id`).
- **Alternatives considered**: Per-user garages (a later change would add an owner FK plus sharing; the
  `created_by` column makes that migration straightforward).
