---

description: "Task list for torq Core Platform"
---

# Tasks: torq Core Platform

**Input**: Design documents from `/specs/001-torq-core-platform/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/rest-api.md, contracts/webhook-payloads.md, quickstart.md

**Tests**: Included. Constitution "Development Workflow & Quality Gates" requires that PRs pass automated tests. Test tasks focus on the parts most likely to break: the reminder engine, odometer rules, money aggregation, auth/session security, webhook presets, and the HTTP contract. Within each phase, tests are listed first.

**Organization**: The phases follow the 7 vertical slices from the `/speckit-tasks` input, so the app builds and can be checked after every phase. Each task carries the spec user story it serves:

| Label | Spec user story |
|---|---|
| US1 | Auth, sign-in & garage |
| US2 | Service Logbook |
| US3 | Reminders & dashboard |
| US4 | Self-Repair & BOM |
| US5 | Spec Sheet |
| US6 | Webhooks |
| US7 | Reports & export |
| US8 | Localization |

**Deviations from the input, and why**:
- **Migrations**: one migration file per phase instead of the whole schema in Phase 1. Each phase stays a self-contained slice, and migrations are forward-only (research R3).
- **Attachments in Phase 5**: all uploads (vehicle avatar, service receipts, spec-entry images, repair files) land in Phase 5, as your input placed "local file attachment upload". Phases 3 and 4 ship without uploads.

**Conventions every task must follow** (from the constitution, research and contracts):
- **Error codes**: the API returns codes, never prose (`contracts/rest-api.md` § Conventions).
- **Translation keys**: every new UI string adds keys to **both** `web/src/i18n/locales/en/*.json` and `web/src/i18n/locales/de/*.json` in the same task.
- **Formatting**: every displayed number, date, currency amount and unit goes through `web/src/lib/format.ts`.
- **Writes**: every write that touches an odometer or the service history runs in one `BEGIN IMMEDIATE` transaction via the writer DB handle.
- **Dark only**: no `dark:` Tailwind variants and no light styles (`print:` is allowed only in the history print route).
- **Phase end**: after each phase, `make check && make test` must pass and the app must start.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: The user story the task serves (US1–US8)

## Path Conventions

- **Layout**: a single repository. The Go module is at the root (`cmd/`, `internal/`). The SPA is in `web/`. The Go package `web` embeds `web/dist`.
- **Tests**: Go tests sit next to the code (`*_test.go`). Shared helpers are in `internal/testutil/`. Frontend unit tests use `web/src/**/*.test.ts(x)`. End-to-end tests are in `web/e2e/`.

---

## Phase 1: Foundation & Storage

**Purpose**: Go module, SQLite (WAL) with embedded migrations, sqlc, HTTP skeleton, and a Vite/React/Tailwind dark shell embedded in the binary.

### Setup

- [X] T001 Initialize the Go module `github.com/<owner>/torq` with `go 1.23` in go.mod and create the directory skeleton from plan.md § Project Structure (`cmd/torq/`, `internal/{config,db,store,httpapi,auth,vehicles,servicelog,repairs,reminders,notify,reports,attachments,scheduler,i18n,testutil}/`, `web/`). Add a `.gitignore` covering `.data/`, `web/dist/*` (keep `web/dist/.gitkeep`), `web/node_modules/`, `coverage/`, and `*.db*`.
- [X] T002 Add the backend dependencies to go.mod/go.sum: `modernc.org/sqlite`, `github.com/pressly/goose/v3`, `golang.org/x/crypto`, `golang.org/x/text`. Add no other runtime dependencies (research R1, R14 and Constitution VI).
- [X] T003 [P] Scaffold the Vite + React 19 + TypeScript SPA in web/ (`web/package.json`, `web/vite.config.ts`, `web/index.html`, `web/src/main.tsx`). Runtime dependencies: `react`, `react-dom`, `react-router`, `@tanstack/react-query`, `i18next`, `react-i18next`, `lucide-react`. Dev dependencies: `tailwindcss` + `@tailwindcss/vite` v4, `typescript`, `vitest`, `@testing-library/react`, `@testing-library/user-event`, `jsdom`, `eslint`, `typescript-eslint`, `eslint-plugin-i18next`, `@playwright/test`. Configure the Vite dev proxy so `/api` and `/healthz` go to `http://localhost:8080`.
- [X] T004 [P] Configure strict TypeScript in web/tsconfig.json (`strict`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `noImplicitOverride`, `verbatimModuleSyntax`), and ESLint in web/eslint.config.js (`typescript-eslint` strict-type-checked, plus `i18next/no-literal-string` in `mode: 'jsx-text-only'` as an error).
- [X] T005 [P] Configure Go linting in .golangci.yml (enable errcheck, govet, staticcheck, gosec, revive, unused, ineffassign, and `gofmt`/`goimports` formatting).
- [X] T006 Create the Makefile with these targets:
  - `setup`: `go mod download` and `npm ci --prefix web`
  - `generate`: `sqlc generate`
  - `dev`: runs `go run ./cmd/torq` with `TORQ_DATA_DIR=./.data TORQ_LOG_FORMAT=text` and `npm run dev --prefix web` concurrently
  - `build-web`: `npm run build --prefix web`
  - `build`: `build-web` then `CGO_ENABLED=0 go build -trimpath -o bin/torq ./cmd/torq`
  - `check`: `go vet`, `golangci-lint run`, a `sqlc diff` that fails on any diff, `npm run typecheck`, `npm run lint`, `npm run i18n:check`
  - `test`: `go test ./...` and `npm test --prefix web -- --run`
  - `e2e`: placeholder target
- [X] T007 [P] Create the CI workflow .github/workflows/ci.yml. On pull requests and pushes it sets up Go (stable) and Node 22, installs sqlc 1.27+ and golangci-lint, then runs `make setup check test build`.

### Foundational (blocks every later phase)

- [X] T008 Implement environment configuration in internal/config/config.go. Use the variables and defaults from research.md R20:
  - `TORQ_DATA_DIR`: default `/data`
  - `TORQ_LISTEN_ADDR`: default `:8080`
  - `TORQ_BASE_URL`: default empty, must be an absolute http(s) URL when set
  - `TORQ_COOKIE_SECURE`: `auto`/`true`/`false`
  - `TORQ_TRUSTED_PROXIES`: comma-separated CIDRs
  - `TORQ_MAX_UPLOAD_MB`: default `25`
  - `TORQ_LOG_LEVEL`: default `info`
  - `TORQ_LOG_FORMAT`: `json`/`text`, default `json`

  `Load()` returns a typed `Config` or a descriptive error.
- [X] T009 [P] Write table tests for config parsing (defaults, invalid CIDR, invalid base URL, upload MB ≤ 0) in internal/config/config_test.go.
- [X] T010 Implement `db.Open(dataDir)` in internal/db/db.go:
  - It creates `dataDir`, then opens `<dataDir>/torq.db` with `modernc.org/sqlite` and returns `DB{Writer, Reader *sql.DB}`.
  - Writer: `SetMaxOpenConns(1)`. Reader pool: `SetMaxOpenConns(4)`.
  - Every connection applies the DSN pragmas `journal_mode(WAL)`, `foreign_keys(ON)`, `busy_timeout(5000)`, `synchronous(NORMAL)`.
  - Add a `WithTx(ctx, writer, fn func(*sql.Tx) error)` helper that issues `BEGIN IMMEDIATE` (research R2).
- [X] T011 Implement `db.Migrate(ctx, DB, dataDir)` in internal/db/migrate.go:
  - Use goose with an `embed.FS` of internal/db/migrations/*.sql.
  - When migrations are pending and the DB already has at least one applied version, first `VACUUM INTO '<dataDir>/backups/torq-<fromVersion>-<UTC timestamp>.db'` and keep only the newest 3 backups.
  - Apply migrations Up only. Never call Down (research R3).
- [X] T012 Create the first migration internal/db/migrations/00001_instance_settings.sql, with only an `-- +goose Up` section:
  - Table `instance_settings` with:
    - `id INTEGER PRIMARY KEY CHECK (id = 1)`
    - `weekly_digest_weekday INTEGER NOT NULL DEFAULT 1 CHECK (weekly_digest_weekday BETWEEN 1 AND 7)`
    - `weekly_digest_hour INTEGER NOT NULL DEFAULT 8 CHECK (weekly_digest_hour BETWEEN 0 AND 23)`
    - `monthly_digest_day INTEGER NOT NULL DEFAULT 1 CHECK (monthly_digest_day BETWEEN 1 AND 28)`
    - `monthly_digest_hour INTEGER NOT NULL DEFAULT 8 CHECK (monthly_digest_hour BETWEEN 0 AND 23)`
  - Seed the row with `INSERT INTO instance_settings (id) VALUES (1)`.
- [X] T013 Configure sqlc in sqlc.yaml: engine `sqlite`, schema `internal/db/migrations`, queries `internal/store/queries`, output package `store` in internal/store, `emit_json_tags: false`, `emit_empty_slices: true`. Add internal/store/queries/instance_settings.sql with `GetInstanceSettings` and `UpdateInstanceSettings`, run `sqlc generate`, and commit the generated code.
- [X] T014 [P] Create a test helper in internal/testutil/db.go. `NewDB(t)` opens a fresh temp-dir database through `db.Open` + `db.Migrate` and registers cleanup. `FixedClock(date)` returns a deterministic clock. Add a `Clock` interface (`Now() time.Time`, `Today() Date`) in internal/clock/clock.go and use it everywhere instead of `time.Now`.
- [X] T015 [P] Write tests for internal/db/db_test.go: after Open + Migrate, `PRAGMA journal_mode` returns `wal` and `PRAGMA foreign_keys` returns `1`. Running Migrate a second time is a no-op. A pending migration on an existing DB creates exactly one backup file.
- [X] T016 Implement the API error model in internal/httpapi/errors.go:
  - Type `APIError{Status int; Code string; Fields map[string]string; Params map[string]any}`.
  - `writeError(w, err)` renders `{"error":{"code","fields?","params?"}}`.
  - Helper constructors: `badRequest`, `unauthenticated`, `forbidden`, `notFound`, `conflict(code, params)`, `validation(fields)`, `tooLarge`, `unsupportedType`, `throttled`.
  - Add `writeJSON(w, status, v)` and `decodeJSON(r, dst)`, which uses `DisallowUnknownFields`, caps the body at 1 MiB, and maps errors to `request.malformed`.
- [X] T017 Implement middleware in internal/httpapi/middleware.go:
  - `Recover`: logs the panic with slog and returns 500 `internal`.
  - `RequestLog`: method, path, status, duration and request id.
  - `CSRF`: for any non-GET/HEAD request under `/api/v1/`, require the header `X-Torq-Request: 1`, and reject an `Origin` header whose host doesn't match the request Host or `TORQ_BASE_URL`, with 403 `csrf.origin_mismatch` (research R6).
  - `SecurityHeaders`: `X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin`, `X-Frame-Options: DENY`, and a CSP of `default-src 'self'; img-src 'self' blob: data:; style-src 'self' 'unsafe-inline'`.
- [X] T018 Implement the router and server in internal/httpapi/router.go:
  - `NewRouter(deps Deps) http.Handler` uses stdlib `http.ServeMux` with method patterns.
  - It mounts `GET /healthz`, which returns `{"status":"ok","version":…}` after `SELECT 1` on the reader, or 503.
  - It mounts the `/api/v1/` subtree, which returns 404 `not_found` JSON for unknown API routes.
  - Everything else goes to the SPA handler.
  - `Deps` holds `Config`, `DB`, `*store.Queries` (reader/writer), `Clock` and `*slog.Logger`.
- [X] T019 Create the embedded SPA package web/embed.go (`package web`, `//go:embed all:dist`, exported `Dist fs.FS`). Implement the SPA handler in internal/httpapi/spa.go:
  - Serve real files from `web.Dist`, with `Cache-Control: public, max-age=31536000, immutable` for `/assets/*`.
  - Fall back to `index.html` with `Cache-Control: no-cache` for any other non-`/api` GET path.
- [X] T020 Implement the entry point in cmd/torq/main.go:
  - Load config and build the slog logger (JSON or text) from it.
  - Import `_ "time/tzdata"` so `TZ` works on distroless.
  - Call `db.Open`, then `db.Migrate`, then build the router.
  - Start `http.Server` with read-header timeout 10 s, idle timeout 120 s and graceful shutdown on SIGINT/SIGTERM.
  - Add the subcommand `torq healthcheck`, which GETs `http://127.0.0.1<port>/healthz` and exits 0 or 1.
- [X] T021 [P] Write HTTP tests in internal/httpapi/router_test.go:
  - `GET /healthz` returns 200 JSON.
  - `GET /api/v1/nope` returns 404 with code `not_found`.
  - `POST /api/v1/x` without `X-Torq-Request` returns 403.
  - `GET /some/spa/route` returns index.html.
- [X] T022 [P] Build the dark-only Tailwind base in web/src/index.css:
  - `@import "tailwindcss";`
  - On `html`: `color-scheme: dark;`, `bg-zinc-950 text-zinc-100 antialiased`.
  - A `focus-visible` ring utility using `ring-sky-400`.
  - No `dark:` variants, no `prefers-color-scheme` queries.
  - Bundled system font stack only, with no external fonts (Constitution III).
- [X] T023 [P] Implement the fetch wrapper in web/src/lib/api.ts:
  - Typed `api.get/post/put/patch/del<T>()` methods.
  - Sends `credentials: 'same-origin'`, plus `X-Torq-Request: 1` on non-GET requests.
  - Parses `{error:{code,fields,params}}` into an `ApiError` class.
  - On a 401 it dispatches a `torq:unauthenticated` window event.
  - `upload<T>(path, FormData)` handles multipart requests.
- [X] T024 [P] Create web/src/lib/queryKeys.ts with centralized React Query key factories (`vehicles`, `vehicle(id)`, `specs(id)`, `serviceLogs(filters)`, `repairs(filters)`, `reminders(filters)`, `dashboard`, `reports(params)`, `webhooks`, `me`, `users`).
- [X] T025 Create the app shell in web/src/app/App.tsx and web/src/app/Layout.tsx:
  - Set up `QueryClientProvider` and the `react-router` data router.
  - The layout has a sidebar on desktop and a top bar with a drawer below `md`.
  - Nav items are empty for now. The app renders the "torq" wordmark only (no other literal strings).
  - Mount it from web/src/main.tsx.
- [X] T026 Add the `npm` scripts in web/package.json:
  - `dev`, `build` (`tsc -b && vite build`), `typecheck`, `lint`, `test` (vitest), `e2e` (playwright), `i18n:check` (placeholder that exits 0 until T040)
- [X] T027 Verify the slice end to end: `make build` produces `bin/torq`. Running it with `TORQ_DATA_DIR=./.data` serves the shell at `/`, `curl /healthz` returns ok, and `./.data/torq.db` plus its `-wal` file exist. Record the commands in README.md § Development.

**Checkpoint**: `make check test build` is green, and the binary serves the dark shell plus `/healthz` from a WAL SQLite database.

---

## Phase 2: Extensible Auth & Localization Settings (US1 auth part, US8)

**Goal**: First-boot admin setup, login/logout, password change, admin user management, per-user localization preferences, and frontend i18n with formatters.

**Independent Test**: quickstart V1 steps 1–2 and 6, plus V8 step 1 for formatters on a sample page. On an empty `/data`, create an admin, restart (no setup offered again), log out and in, change the password (the other session gets 401), switch to German / `DD.MM.YYYY` / comma decimals and see German UI text.

### Tests

- [X] T028 [P] [US1] Write argon2id tests in internal/auth/local/password_test.go:
  - hash and verify round trip
  - a wrong password fails
  - the PHC string format is `$argon2id$v=19$m=19456,t=2,p=1$…`
  - `NeedsRehash` is true for hashes with other parameters
- [X] T029 [P] [US1] Write session tests in internal/auth/sessions_test.go:
  - the token is 32 random bytes and the DB stores only `sha256(token)`
  - lookup with an expired token fails
  - `last_seen_at` refreshes at most hourly and extends `expires_at` by 30 days
  - `DeleteOthers(userID, keepID)` removes the other sessions
- [X] T030 [P] [US1] Write HTTP contract tests for setup and auth in internal/httpapi/auth_test.go:
  - `GET /setup/status` is true on an empty DB. `POST /setup` returns 201 and sets the cookie. A second `POST /setup` returns 409 `setup.already_completed`.
  - Login with a wrong password and login with an unknown user both return 401 `auth.invalid_credentials` with identical bodies.
  - After 5 failures the response is 429 `auth.throttled` with `retry_after_s`.
  - `/auth/me` without a cookie returns 401.
  - `POST /auth/password` revokes the other sessions.
  - Users admin: `PATCH` that demotes the last admin returns 409 `user.last_admin`, and deleting yourself returns 409 `user.cannot_delete_self`.

### Implementation

- [X] T031 [US1] Create migration internal/db/migrations/00002_identity.sql with the following tables, exactly as in data-model.md § 1.

  **`users`**:
  - `username TEXT NOT NULL UNIQUE COLLATE NOCASE`
  - `display_name TEXT NOT NULL`
  - `role TEXT NOT NULL CHECK (role IN ('admin','member'))`
  - `locale TEXT NOT NULL DEFAULT 'en' CHECK (locale IN ('en','de'))`
  - `date_format TEXT NOT NULL DEFAULT 'YYYY-MM-DD' CHECK (date_format IN ('DD.MM.YYYY','YYYY-MM-DD'))`
  - `number_format TEXT NOT NULL DEFAULT 'point' CHECK (number_format IN ('point','comma'))`
  - `currency_code TEXT NOT NULL DEFAULT 'EUR'`
  - `currency_symbol TEXT NULL CHECK (currency_symbol IS NULL OR length(currency_symbol) <= 5)`
  - `disabled_at TEXT NULL`
  - `created_at`, `updated_at`

  **`user_identities`**:
  - `user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE`
  - `provider TEXT NOT NULL`
  - `subject TEXT NOT NULL`
  - `issuer TEXT NULL`
  - `password_hash TEXT NULL`
  - `CHECK ((provider = 'local') = (password_hash IS NOT NULL))`
  - `UNIQUE (provider, subject)`
  - a partial unique index on `user_id WHERE provider = 'local'`
  - `created_at`, `last_login_at`

  **`sessions`**:
  - `user_id … ON DELETE CASCADE`
  - `token_hash BLOB NOT NULL UNIQUE`
  - `created_at`, `last_seen_at`, `expires_at TEXT NOT NULL`
  - `user_agent TEXT NOT NULL DEFAULT ''`
  - `ip TEXT NOT NULL DEFAULT ''`
- [X] T032 [US1] Write sqlc queries in internal/store/queries/users.sql:
  - `CountUsers`, `CreateUser`, `GetUser`, `GetUserByUsername`, `ListUsers`
  - `UpdateUserProfile`, `UpdateUserPreferences`, `SetUserRole`, `SetUserDisabled`, `DeleteUser`, `CountEnabledAdmins`
  - `CreateLocalIdentity`, `GetLocalIdentityByUsername` (joins users, filters `disabled_at IS NULL`), `UpdatePasswordHash`, `UpdateLocalSubject`, `TouchLastLogin`

  Write sqlc queries in internal/store/queries/sessions.sql:
  - `CreateSession`, `GetSessionByTokenHash` (joins users)
  - `TouchSession`, `DeleteSession`, `DeleteUserSessions`, `DeleteUserSessionsExcept`, `DeleteExpiredSessions`

  Then run `sqlc generate`.
- [X] T033 [P] [US1] Implement argon2id in internal/auth/local/password.go: `Hash(pw)`, `Verify(hash, pw)` (constant-time compare), and `NeedsRehash(hash)`. Parameters: m=19456 KiB, t=2, p=1, 16-byte salt, 32-byte key (research R5).
- [X] T034 [US1] Implement the provider boundary in internal/auth/auth.go:
  - Type `User{ID, Username, DisplayName, Role, Prefs}` and type `Preferences{Locale, DateFormat, NumberFormat, CurrencyCode, CurrencySymbol}`.
  - `UserFrom(ctx)`, and `WithUser(ctx, u)`, which is unexported to the domain.
  - `type Provider interface{ Kind() string }` and `type PasswordAuthenticator interface{ Provider; AuthenticatePassword(ctx, username, password string) (int64, error) }`.

  Implement `local.Provider` in internal/auth/local/provider.go using `GetLocalIdentityByUsername`:
  - Rehash on login when `NeedsRehash` is true.
  - For unknown users, run a dummy `Verify` against a fixed hash so timing doesn't reveal whether the user exists.
  - Domain packages must never import `auth/local` (Constitution IV).
- [X] T035 [US1] Implement sessions in internal/auth/sessions.go:
  - `Create(ctx, userID, ua, ip)` returns a raw token from 32 `crypto/rand` bytes, base64url-encoded, and stores the SHA-256 of it.
  - `Lookup(ctx, rawToken)`.
  - `Touch`: refresh when `last_seen_at` is older than 1 h, setting `expires_at = now + 30 days`.
  - `Revoke`, `RevokeAllExcept`.
  - Cookie helpers: name `torq_session`, `HttpOnly`, `SameSite=Lax`, `Path=/`. `Secure` is set when `TORQ_COOKIE_SECURE=true`, or when it is `auto` and `TORQ_BASE_URL` starts with `https://`.
- [X] T036 [US1] Implement the in-memory login throttle in internal/auth/throttle.go:
  - Keyed by lowercase username and by client IP.
  - After 5 consecutive failures, delay grows exponentially from 1 s up to a 15 min lockout. Success resets the counter.
  - `ClientIP(r, trustedProxies)` honors `X-Forwarded-For` only when the peer address is inside a `TORQ_TRUSTED_PROXIES` CIDR (research R8).
- [X] T037 [US1] Implement auth middleware in internal/auth/middleware.go:
  - `Authenticate` reads the cookie, looks up and touches the session, and attaches the `User` to the context. An invalid cookie is simply ignored.
  - `RequireUser` returns 401 `auth.unauthenticated`.
  - `RequireAdmin` returns 403 `auth.forbidden`.
  - Wire all three into internal/httpapi/router.go, so every `/api/v1/*` route except `GET /setup/status`, `POST /setup` and `POST /auth/login` requires a user.
- [X] T038 [US1] Implement the setup and auth handlers in internal/httpapi/auth.go, following contracts/rest-api.md § Setup & Auth:
  - `GET /api/v1/setup/status`, and `POST /api/v1/setup` (409 `setup.already_completed` when any user exists; creates an admin plus local identity in one transaction and sets the cookie).
  - `POST /api/v1/auth/login`, `POST /api/v1/auth/logout`, `GET /api/v1/auth/me`.
  - `POST /api/v1/auth/password`: verify the current password, then `RevokeAllExcept`.
  - `PATCH /api/v1/me/preferences`.
  - Validation codes for `username` (3–32 chars, `[a-zA-Z0-9._-]`), `display_name` (1–64 chars) and password (≥ 10 chars, `validation.password_too_short`).
  - Prune expired sessions on startup in cmd/torq/main.go.
- [X] T039 [US1] Implement admin user management in internal/httpapi/users.go and internal/auth/users.go:
  - `GET/POST /api/v1/users` and `PATCH/DELETE /api/v1/users/{id}`.
  - `POST /api/v1/users/{id}/password` revokes all of that user's sessions.
  - The "at least one enabled admin" invariant is checked inside the transaction via `CountEnabledAdmins`.
  - Disabling a user deletes their sessions. Renaming updates the local identity `subject` in the same transaction (data-model § 1).
- [X] T040 [P] [US8] Set up i18next in web/src/i18n/index.ts:
  - Bundled resources, no HTTP backend.
  - Namespaces `common, auth, vehicles, specs, service, repairs, reminders, reports, settings, webhooks, errors` in web/src/i18n/locales/en/*.json and web/src/i18n/locales/de/*.json.
  - `fallbackLng: 'en'`, `returnNull: false`.
  - Add web/scripts/check-i18n.ts, which fails when any `en` key is missing from `de`, and wire it to the `i18n:check` script (research R16).
- [X] T041 [P] [US8] Implement the formatters in web/src/lib/format.ts as `createFormatter(prefs)`:
  - `number(n, {decimals})`: `Intl.NumberFormat` with `de-DE` for `comma` and `en-US` for `point`.
  - `money(cents)`: `Intl.NumberFormat` with style `currency` and `currencyDisplay: 'narrowSymbol'`. A custom `currency_symbol` replaces the `currency` part via `formatToParts`.
  - `date(isoDate)`: `Intl.DateTimeFormat` `formatToParts`, reordered to `DD.MM.YYYY` or `YYYY-MM-DD`.
  - Units: `km(n)`, `liters(ml)` (shows L with up to 2 decimals), `ml(n)`, `nm(n)`, `bar(n)`, with suffixes from the i18n keys `common:units.*`.
  - No unit conversion anywhere.
  - Export a `useFormat()` hook in web/src/lib/useFormat.ts that reads the current user's preferences.
- [X] T042 [P] [US8] Write formatter tests in web/src/lib/format.test.ts:
  - Comma format: `215000` → `215.000 km`, `8950` cents with EUR → `89,50 €`, `2026-10-15` with `DD.MM.YYYY` → `15.10.2026`.
  - Point format: `215,000 km` and `$89.50`.
  - A custom symbol `CHF` replaces the currency part.
  - `6500` mL → `6,5 L` (comma) or `6.5 L` (point).
- [X] T043 [US1] Build the auth UI:
  - `AuthGate` in web/src/app/AuthGate.tsx queries `/setup/status` and `/auth/me`, routes to Setup or Login, listens for `torq:unauthenticated`, and applies the user's locale via `i18n.changeLanguage`.
  - Pages: web/src/features/auth/SetupPage.tsx, web/src/features/auth/LoginPage.tsx, web/src/features/auth/ChangePasswordPage.tsx.
  - A logout action in the Layout user menu.
  - Error codes map to `errors:*` translations. Add the en and de keys.
- [X] T044 [US8] Build the preferences page web/src/features/settings/PreferencesPage.tsx:
  - Fields: display name, language (`en`, `de`), date format (`DD.MM.YYYY`, `YYYY-MM-DD`), number format (`point` 1,234.5 / `comma` 1.234,5), currency code (EUR, USD, GBP, CHF) and an optional custom symbol (≤ 5 chars).
  - Saves via `PATCH /me/preferences`, invalidates `queryKeys.me`, and switches language immediately.
  - Add a live preview line rendered through `useFormat()`.
- [X] T045 [US1] Build the admin users page web/src/features/settings/UsersPage.tsx: list, create (username, display name, password, role), disable/enable, reset password, delete. Show the 409 codes `user.last_admin` and `user.cannot_delete_self` as translated messages. Only admins see it, and it sits under a Settings nav section in web/src/app/Layout.tsx.

**Checkpoint**: A fresh instance runs the setup → login → logout → password change flow. Admins manage users. Switching to German and comma format changes all visible text and formats. `make check` (including `i18n:check`) is green.

---

## Phase 3: Vehicle Garage & Quick-Reference Spec Sheet (US1 garage part, US5)

**Goal**: Vehicle CRUD with archive, odometer quick-update with readings history and the confirmation rules, the spec sheet with typed fields and custom entries, a dashboard with vehicle cards, and the ⌘K palette (odometer action).

**Independent Test**: quickstart V1 steps 3–5 and V5 steps 1–2, without the image (images arrive in Phase 5). Add the S203 at 214,500 km, quick-update to 215,200, check that a decrease to 21,520 asks for confirmation, and fill in the spec sheet (5W-30 / MB 229.51 / 6.5 L, 150 Nm, 2.3/2.5 bar).

### Tests

- [X] T046 [P] [US1] Write odometer rule tests in internal/vehicles/odometer_test.go:
  - Automatic sources (`service_log`, `repair_completion`, `reminder_done`) raise `current_odometer_km` and never lower it.
  - A quick-update below current returns the conflict `odometer.decrease_requires_confirmation` (`params.current_km`) unless `confirm_decrease` is set.
  - A quick-update that more than doubles the value or adds > 100,000 km needs `confirm_large_jump`.
  - Every call inserts an `odometer_readings` row with the right `source`.
- [X] T047 [P] [US1] Write vehicle contract tests in internal/httpapi/vehicles_test.go:
  - create, get, patch and list (with `archived=false|true|all`)
  - archive and unarchive
  - `DELETE` with a mismatched `confirm_name` returns 422 `vehicle.confirm_name_mismatch`
  - an invalid VIN returns 422 with `fields.vin`
  - `POST /vehicles/{id}/odometer` returns 409 codes as in T046
  - `PUT /vehicles/{id}/spec-entries/order` with a missing ID returns 422

### Implementation

- [X] T048 [US1] Create migration internal/db/migrations/00003_vehicles.sql, following data-model.md § 2.

  **`vehicles`**:
  - `make TEXT NOT NULL`, `model TEXT NOT NULL` (1–64 chars)
  - `chassis_code TEXT NOT NULL DEFAULT ''`
  - `model_year INTEGER NULL`
  - `engine_code TEXT NOT NULL DEFAULT ''`
  - `vin TEXT NULL CHECK (vin IS NULL OR length(vin) = 17)`
  - `license_plate TEXT NULL CHECK (license_plate IS NULL OR length(license_plate) <= 16)`
  - `current_odometer_km INTEGER NOT NULL CHECK (current_odometer_km >= 0)`
  - `archived_at TEXT NULL`
  - `created_by INTEGER NULL REFERENCES users(id) ON DELETE SET NULL`
  - `created_at`, `updated_at`

  **`odometer_readings`**:
  - `vehicle_id … ON DELETE CASCADE`
  - `reading_date TEXT NOT NULL`
  - `odometer_km INTEGER NOT NULL CHECK (odometer_km >= 0)`
  - `source TEXT NOT NULL CHECK (source IN ('initial','quick_update','service_log','repair_completion','reminder_done'))`
  - `created_at`
  - index `(vehicle_id, reading_date, odometer_km)`
  - `service_log_id` and `repair_log_id` are added later by ALTER in 00004 and 00005.

  **`vehicle_specs`**:
  - `vehicle_id INTEGER PRIMARY KEY REFERENCES vehicles(id) ON DELETE CASCADE`
  - the oil, transmission, differential, brake and coolant `*_spec`/`*_viscosity` columns as `TEXT NOT NULL DEFAULT ''`
  - the `*_capacity_ml` columns as `INTEGER NULL CHECK (> 0)`
  - `wheel_lug_torque_nm REAL NULL CHECK (wheel_lug_torque_nm > 0)`
  - `tire_pressure_front_bar` and `tire_pressure_rear_bar` as `REAL NULL CHECK (> 0 AND <= 10)`
  - `updated_at`

  **`vehicle_spec_entries`**:
  - `vehicle_id … ON DELETE CASCADE`
  - `label TEXT NOT NULL CHECK (length(label) BETWEEN 1 AND 80)`
  - `value TEXT NOT NULL DEFAULT '' CHECK (length(value) <= 2000)`
  - `sort_order INTEGER NOT NULL`
  - `created_at`, `updated_at`
- [X] T049 [US1] Write sqlc queries in internal/store/queries/vehicles.sql:
  - `CreateVehicle`, `GetVehicle`, `ListVehicles` (with the `sqlc.narg('archived')` filter), `UpdateVehicleProfile`, `SetVehicleArchived`, `DeleteVehicle`, `SetCurrentOdometer`
  - `InsertOdometerReading`, `ListOdometerReadings` (with from/to)
  - `CreateVehicleSpecs`, `GetVehicleSpecs`, `UpsertVehicleSpecs`
  - `ListSpecEntries`, `CreateSpecEntry`, `UpdateSpecEntry`, `DeleteSpecEntry`, `SetSpecEntryOrder`, `MaxSpecEntryOrder`

  Then run `sqlc generate`.
- [X] T050 [US1] Implement the vehicle domain in internal/vehicles/vehicles.go:
  - `Create`, in one transaction, inserts the vehicle, an empty `vehicle_specs` row and an `initial` reading.
  - `Update`, `Archive`/`Unarchive`, and `Delete(ctx, id, confirmName)`, which must match `make + " " + model` case-insensitively.
  - Validation:
    - `make`/`model` 1–64 chars
    - `model_year` between 1886 and the current year + 1
    - VIN uppercased, exactly 17 chars, `[A-HJ-NPR-Z0-9]`
    - license plate ≤ 16 chars
    - odometer ≥ 0
- [X] T051 [US1] Implement `RecordReading(ctx, tx, vehicleID, date, km, source, refs)` and `QuickUpdate(ctx, vehicleID, km, date, confirmDecrease, confirmLargeJump)` in internal/vehicles/odometer.go, following the data-model.md § 2 odometer rules 1–5. Export a hook point `AfterOdometerChange func(ctx, tx, vehicleID) ([]ReminderSummary, error)`, a no-op until Phase 4, which the handlers include in responses as `reminders`.
- [X] T052 [P] [US5] Implement the spec sheet domain in internal/vehicles/specs.go:
  - `GetSpecs`, and `PutSpecs`, which does a full replace and validates `capacity_ml > 0`, `torque_nm > 0` and `0 < bar <= 10`.
  - Entry CRUD: label 1–80 chars, value ≤ 2000 chars. New entries get `sort_order = max + 1`.
  - `Reorder(ids)`, which must contain every entry exactly once and rewrites a contiguous `sort_order`.
- [X] T053 [US1] Implement the vehicle handlers in internal/httpapi/vehicles.go, following contracts/rest-api.md § Vehicles & Odometer: `GET/POST /vehicles`, `GET/PATCH/DELETE /vehicles/{id}`, `POST /vehicles/{id}/archive|unarchive`, `POST /vehicles/{id}/odometer`, `GET /vehicles/{id}/odometer-readings`. The `VehicleSummary` includes `reminder_counts: {due_soon: 0, overdue: 0}`, which is filled in Phase 4, and `avatar_url: null`, which is filled in Phase 5.
- [X] T054 [P] [US5] Implement the spec handlers in internal/httpapi/specs.go, following contracts/rest-api.md § Spec sheet: `GET/PUT /vehicles/{id}/specs`, `POST /vehicles/{id}/spec-entries`, `PATCH/DELETE /spec-entries/{id}`, `PUT /vehicles/{id}/spec-entries/order`.
- [X] T055 [P] [US1] Build reusable UI components in web/src/components/:
  - `Button.tsx`, `Field.tsx` (label, error code → translated message), `NumberInput.tsx` (locale-aware parsing of comma or point decimals, with a unit suffix slot)
  - `Dialog.tsx`, `ConfirmDialog.tsx` (with an optional "type to confirm" input)
  - `Badge.tsx` (states: upcoming = zinc, due_soon = amber, overdue = red, completed/installed = emerald)
  - `EmptyState.tsx`, `Table.tsx`

  All are dark-only with visible focus rings.
- [X] T056 [US1] Build the vehicle pages:
  - web/src/features/vehicles/VehicleListPage.tsx: cards with the placeholder avatar, make/model/chassis code, odometer via `fmt.km`, and an archived filter.
  - web/src/features/vehicles/VehicleFormPage.tsx: create and edit.
  - web/src/features/vehicles/VehicleDetailPage.tsx: tabs for Overview and Spec Sheet, with Service, Repairs and Reminders tabs added in later phases. Archive/unarchive and delete use the type-the-name `ConfirmDialog`.
  - Add the vehicle switcher to web/src/app/Layout.tsx.
  - Add the `vehicles` en/de keys.
- [X] T057 [US1] Build the odometer quick-update component web/src/features/vehicles/OdometerQuickUpdate.tsx. It submits to `/vehicles/{id}/odometer`. On 409 `odometer.decrease_requires_confirmation` or `odometer.large_jump_requires_confirmation` it shows a `ConfirmDialog` with the current and new km and resubmits with the confirm flag. On success it invalidates `vehicle(id)`, `vehicles`, `dashboard` and `reminders`.
- [X] T058 [US1] Build the dashboard web/src/features/dashboard/DashboardPage.tsx: vehicle cards, each with an `OdometerQuickUpdate`. Leave placeholders for the attention list (Phase 4) and open repairs (Phase 5). Make it the index route.
- [X] T059 [US1] Build the quick-action palette web/src/app/QuickActions.tsx:
  - Opens with `Ctrl/⌘+K` or a header button.
  - Has a filterable action list with arrow-key navigation, Enter to run and Esc to close.
  - First action: "Log odometer reading", which picks a vehicle and then opens `OdometerQuickUpdate`.
  - Expose an `registerAction` array so later phases can add actions.
- [X] T060 [US5] Build the spec sheet pinboard web/src/features/vehicles/SpecSheet.tsx:
  - A compact read view grouping engine oil (viscosity · spec · capacity in L), transmission, differential, brake and coolant (spec · capacity L), lug torque (Nm) and tire pressure front/rear (bar).
  - An edit mode with `NumberInput` for the metric values (stored as mL; entered in L with up to 2 decimals).
  - Custom entries with add, edit, delete and reorder via up/down buttons, calling `PUT …/order`.
  - Add the `specs` en/de keys.

**Checkpoint**: The garage, odometer history and spec sheet work end to end. Decreases and large jumps need confirmation. `make check test` is green.

---

## Phase 4: Service Logbook & Reminders Engine (US2, US3)

**Goal**: Categories (11 built-in + custom), multi-item service sessions, reminders (km, time and composite) with automatic recalculation on every service or odometer write, the scheduler tick for time-based states, and the dashboard attention list.

**Independent Test**: quickstart V2 steps 1–3, 5–7 and V3. A 10,000 km / 12 months oil reminder goes Due Soon → Overdue through odometer updates. Logging a 3-item service resets it to Upcoming with the correct next km and date. A back-fill doesn't change it. Deleting the service reverts it.

### Tests

- [X] T061 [P] [US3] Write reminder engine tests (pure, table-driven) in internal/reminders/engine/engine_test.go:
  - `AddMonths` clamping: Jan 31 + 1 month is Feb 28, and Feb 29 in a leap year. Aug 31 + 6 months is Feb 28.
  - `NextDue` for km-only, months-only, days-only and composite intervals.
  - `Evaluate` covers the boundaries exactly:
    - `cur = next_due_km - lead_km` gives due_soon
    - `cur = next_due_km` gives overdue
    - `today = next_due_date - lead_days` gives due_soon
    - `today = next_due_date` gives overdue
  - Composite: the more urgent dimension wins and `Trigger` records it.
  - `remaining_km` and `remaining_days` are negative when overdue.
- [X] T062 [P] [US3] Write transition tests in internal/reminders/evaluate_test.go, using a real DB via testutil:
  - `EvaluateVehicle` returns transitions only when `state` changes.
  - Paused reminders and reminders of archived vehicles are skipped.
  - The baseline for linked reminders is the latest service by `service_date DESC, odometer_km DESC`, falling back to `start_*`.
  - A back-filled older service doesn't change the next due values.
  - Deleting the latest service falls back to the previous baseline.
- [X] T063 [P] [US2] Write service log contract tests in internal/httpapi/service_logs_test.go:
  - Create with 3 items returns `reminders` re-evaluated in the response.
  - A duplicate category in one log returns 422 `service_log.duplicate_category`.
  - Zero items returns 422 `service_log.items_required`.
  - An archived category returns 422 `service_log.category_archived`.
  - An odometer value that doesn't fit the history returns 201 with `warnings: ["odometer.inconsistent_with_history"]`.
  - `PUT` replaces the items.
  - `DELETE` returns the re-evaluated reminders.
  - Filters `vehicle_id`, `category_id`, `from`/`to` and `q` work.
  - Deleting a custom category that is in use returns 409 `category.in_use`.

### Implementation

- [X] T064 [US2] Create migration internal/db/migrations/00004_service_log_reminders.sql, following data-model.md § 3 and § 6.

  **`service_categories`**:
  - `builtin_key TEXT NULL UNIQUE`
  - `custom_name TEXT NULL`
  - `is_fluid INTEGER NOT NULL CHECK (is_fluid IN (0,1))`
  - `default_interval_km INTEGER NULL`, `default_interval_months INTEGER NULL`
  - `sort_order INTEGER NOT NULL`
  - `archived_at TEXT NULL`
  - `CHECK ((builtin_key IS NULL) <> (custom_name IS NULL))`
  - a unique index on `custom_name COLLATE NOCASE WHERE custom_name IS NOT NULL`
  - Seed the 11 built-ins: `engine_oil`, `transmission_fluid`, `brake_fluid`, `coolant`, `differential_fluid` (all `is_fluid=1`), and `air_filter`, `cabin_filter`, `fuel_filter`, `spark_glow_plugs`, `brake_pads_rotors`, `timing_belt_chain` (all `is_fluid=0`).

  **`service_logs`**:
  - `vehicle_id … ON DELETE CASCADE`
  - `service_date TEXT NOT NULL`
  - `odometer_km INTEGER NOT NULL CHECK (odometer_km >= 0)`
  - `notes TEXT NOT NULL DEFAULT '' CHECK (length(notes) <= 10000)`
  - `extra_cost_cents INTEGER NOT NULL DEFAULT 0 CHECK (extra_cost_cents >= 0)`
  - `extra_cost_label TEXT NOT NULL DEFAULT ''`
  - `created_by`, `created_at`, `updated_at`
  - index `(vehicle_id, service_date DESC, odometer_km DESC)`

  **`service_log_items`**:
  - `service_log_id … ON DELETE CASCADE`
  - `category_id INTEGER NOT NULL REFERENCES service_categories(id) ON DELETE RESTRICT`
  - `brand`, `specification`, `oem_part_number`, `aftermarket_part_number` as `TEXT NOT NULL DEFAULT ''`
  - `quantity_ml INTEGER NULL CHECK (quantity_ml > 0)`
  - `quantity_unit TEXT NULL CHECK (quantity_unit IN ('L','mL'))`
  - `CHECK ((quantity_ml IS NULL) = (quantity_unit IS NULL))`
  - `cost_cents INTEGER NULL CHECK (cost_cents >= 0)`
  - `cost_type TEXT NOT NULL CHECK (cost_type IN ('parts','fluids'))`
  - `sort_order INTEGER NOT NULL`
  - `UNIQUE (service_log_id, category_id)`
  - index `(category_id, service_log_id)`

  **`reminders`**: all columns from data-model § 6, including:
  - `service_category_id INTEGER NULL REFERENCES service_categories(id) ON DELETE RESTRICT`
  - `title TEXT NOT NULL DEFAULT ''`
  - `interval_km INTEGER NULL CHECK (interval_km > 0)`
  - `interval_months INTEGER NULL CHECK (interval_months BETWEEN 1 AND 240)`
  - `interval_days INTEGER NULL CHECK (interval_days BETWEEN 1 AND 3650)`
  - `lead_km INTEGER NULL CHECK (lead_km >= 0)`, `lead_days INTEGER NULL CHECK (lead_days >= 0)`
  - `start_date TEXT NOT NULL`, `start_odometer_km INTEGER NOT NULL`
  - `last_done_date`, `last_done_odometer_km`, `next_due_km`, `next_due_date`
  - `state TEXT NOT NULL DEFAULT 'upcoming' CHECK (state IN ('upcoming','due_soon','overdue'))`
  - `state_trigger TEXT NULL CHECK (state_trigger IN ('km','time'))`
  - `state_changed_at TEXT NOT NULL`
  - `last_notified_state TEXT NOT NULL DEFAULT 'upcoming'` with the same CHECK
  - `paused_at TEXT NULL`
  - `CHECK (interval_km IS NOT NULL OR interval_months IS NOT NULL OR interval_days IS NOT NULL)`
  - `CHECK (NOT (interval_months IS NOT NULL AND interval_days IS NOT NULL))`
  - `CHECK (service_category_id IS NOT NULL OR length(title) > 0)`
  - indexes `(vehicle_id, state)` and `(service_category_id)`

  Finally: `ALTER TABLE odometer_readings ADD COLUMN service_log_id INTEGER NULL REFERENCES service_logs(id) ON DELETE CASCADE`.
- [X] T065 [US2] Write sqlc queries:
  - internal/store/queries/service_categories.sql: `ListCategories` (with an `include_archived` narg), `CreateCustomCategory`, `UpdateCategory`, `SetCategoryArchived`, `DeleteCategory`, `CategoryInUse`.
  - internal/store/queries/service_logs.sql: `CreateServiceLog`, `GetServiceLog`, `ListServiceLogs` (nargs for vehicle, category, from, to and q; `q` uses LIKE on notes, brand, specification and part numbers), `CountServiceLogs`, `UpdateServiceLog`, `DeleteServiceLog`, `InsertServiceLogItem`, `DeleteServiceLogItems`, `ListServiceLogItems`, `NeighborServiceLogs` (the nearest earlier and later by date, for the consistency warning), `UpsertServiceLogReading`.

  Then run `sqlc generate`.
- [X] T066 [US2] Implement the categories domain in internal/servicelog/categories.go:
  - Custom name 1–64 chars and unique (NOCASE).
  - Built-ins can only have their `default_interval_*` edited and cannot be archived or deleted.
  - Delete returns 409 `category.in_use` when referenced by items or reminders.
- [X] T067 [US2] Implement the service log domain in internal/servicelog/servicelog.go:
  - `Create`, `Replace` (`PUT`) and `Delete` each run in one writer transaction.
  - Validation:
    - ≥ 1 item
    - no duplicate category
    - category not archived on create
    - `service_date ≤ today`
    - `quantity_unit` required iff `quantity_ml` is set
    - `cost_type` defaults from `category.is_fluid` (1 → `fluids`, 0 → `parts`)
  - Upsert the linked reading via `vehicles.RecordReading(source='service_log')`.
  - Compute the `odometer.inconsistent_with_history` warning from `NeighborServiceLogs`.
  - Call the `vehicles.AfterOdometerChange` hook, now wired to reminders (T071), before commit, and return the re-evaluated reminders.
  - The `servicelog` package must not import `repairs` (Constitution II).
- [X] T068 [US3] Implement the pure engine in internal/reminders/engine/engine.go:
  - Types `Date` (civil date), `Interval{Km, Months, Days *int}`, `Lead{Km, Days *int}` and `Status{State, Trigger, RemainingKm, RemainingDays *int}`.
  - `AddMonths` with end-of-month clamping.
  - `NextDue(baseline, interval)`.
  - `Evaluate(next, lead, curKm, today)`, using the exact formula from data-model.md § 6.
- [X] T069 [US3] Write sqlc queries in internal/store/queries/reminders.sql:
  - `CreateReminder`, `GetReminder`, `ListReminders` (nargs for vehicle, state and include_paused; ordered overdue first, then due_soon, then smallest remaining), `UpdateReminder`, `DeleteReminder`
  - `SetReminderPaused`, `SetReminderDone`, `SetReminderEvaluation` (updates next_due_*, state, state_trigger, state_changed_at, last_notified_state)
  - `ListActiveRemindersForVehicle` (not paused, vehicle not archived), `ListAllActiveReminders`
  - `LatestServiceForCategory` (vehicle_id, category_id → service_date, odometer_km, ordered `service_date DESC, odometer_km DESC LIMIT 1`)
  - `CountReminderStatesByVehicle`

  Then run `sqlc generate`.
- [X] T070 [US3] Implement the reminder domain in internal/reminders/reminders.go:
  - `Create` and `Update`. Validation codes: `reminder.interval_required`, `reminder.months_and_days_exclusive`, `reminder.title_required` (standalone, 1–120 chars), `reminder.lead_exceeds_interval` (`lead_km < interval_km`; `lead_days <` the interval in days, using months × 28 as the lower bound).
  - Defaults: `lead_km` 1,000 (capped at interval − 1), `lead_days` 30, `start_date` today, `start_odometer_km` the current odometer. `interval_*` is prefilled from the category defaults when omitted on a linked reminder.
  - `MarkDone(id, date, km)`, standalone only. Otherwise return 409 `reminder.linked_use_service_log`. It records a `reminder_done` reading.
  - `Pause`, `Resume`, `Delete`.
- [X] T071 [US3] Implement evaluation in internal/reminders/evaluate.go:
  - `EvaluateVehicle(ctx, tx, vehicleID, today)` resolves each active reminder's baseline (linked → `LatestServiceForCategory`, otherwise `start_*`; standalone → `last_done_*`, otherwise `start_*`), computes `NextDue` and `Evaluate`, persists the result, and returns `[]Transition{ReminderID, From, To}`.
  - It applies the data-model § 6 `last_notified_state` rules. A move to upcoming resets it to `upcoming`. A move from overdue to due_soon sets `due_soon` without a transition event.
  - `EvaluateAll(ctx, today)` loops over the vehicles.
  - Set `vehicles.AfterOdometerChange` to call `EvaluateVehicle`. Also call it from archive and unarchive, and from reminder create and update.
  - Define the `notify` hook `OnTransitions func(ctx, tx, []Transition) error` as a no-op until Phase 6.
- [X] T072 [US3] Implement the scheduler in internal/scheduler/scheduler.go:
  - `Run(ctx)` runs `reminders.EvaluateAll` plus session pruning once at startup and then on a 15-minute `time.Ticker` (research R11).
  - Each step gets its own error logging so one failure doesn't stop the others.
  - Start it from cmd/torq/main.go and stop it on shutdown.
- [X] T073 [US2] Implement the category and service log handlers in internal/httpapi/service_categories.go and internal/httpapi/service_logs.go, following contracts/rest-api.md § Service categories and § Service logbook. Responses include `reminders` and `warnings` where specified.
- [X] T074 [US3] Implement the reminder and dashboard handlers in internal/httpapi/reminders.go and internal/httpapi/dashboard.go, following contracts/rest-api.md § Reminders & dashboard:
  - The `Reminder` JSON includes `baseline`, `remaining_km`, `remaining_days`, `state`, `state_trigger` and `paused`.
  - `GET /dashboard` returns `vehicles` (with `reminder_counts` from `CountReminderStatesByVehicle`), `attention` (overdue then due_soon) and `open_repairs: []`, which is filled in Phase 5.
  - Update `VehicleSummary.reminder_counts` in internal/httpapi/vehicles.go.
- [X] T075 [US2] Build the service log UI:
  - web/src/features/service-log/ServiceLogListPage.tsx: filters for vehicle, category, date range and search. Rows show the date, km, the category chips and the total.
  - web/src/features/service-log/ServiceLogForm.tsx: date, km, notes, an extra cost and label, and a dynamic list of items. Each item has a category select (built-in names via `t('service:categories.'+key)`), brand, specification, quantity with an L/mL toggle (stored as mL), OEM/aftermarket part numbers, cost via `NumberInput` → cents, and a cost-type select that defaults from `is_fluid`.
  - Show `warnings` as a non-blocking banner.
  - On save, invalidate `serviceLogs`, `vehicle`, `reminders` and `dashboard`.
  - web/src/features/service-log/ServiceLogDetailPage.tsx.
  - Add the `service` en/de keys, including all 11 category names (de: "Motoröl & Filter", "Getriebeöl", "Bremsflüssigkeit", "Kühlmittel", "Differenzialöl", "Luftfilter", "Innenraumfilter", "Kraftstofffilter", "Zünd-/Glühkerzen", "Bremsbeläge/-scheiben", "Zahnriemen/Steuerkette").
- [X] T076 [US2] Build custom category management in web/src/features/settings/CategoriesPage.tsx: list built-in and custom categories, create, rename, edit default intervals, archive/unarchive, and delete (show `category.in_use` as a translated message).
- [X] T077 [US5] Show spec hints in web/src/features/service-log/SpecHint.tsx. When an item's category is `engine_oil`, `transmission_fluid`, `differential_fluid`, `brake_fluid` or `coolant`, show the vehicle's stored spec and capacity (e.g. "5W-30 · MB 229.51 · 6,5 L") under the item, loaded from `queryKeys.specs(vehicleId)` (FR-016).
- [X] T078 [US3] Build the reminder UI:
  - web/src/features/reminders/ReminderListPage.tsx: grouped by state with `Badge` colors, showing remaining or exceeded km and days via `fmt`.
  - web/src/features/reminders/ReminderForm.tsx: linked category or standalone title, a km interval, a time interval (months or days, which are mutually exclusive), lead km and days, start date and odometer.
  - A "Mark done" dialog for standalone reminders, plus pause/resume and delete.
  - Add the `reminders` en/de keys.
- [X] T079 [US3] Add the dashboard attention list in web/src/features/dashboard/AttentionList.tsx (overdue first, then due soon, with vehicle name, reminder name, remaining or exceeded km/days, and the trigger), wired into DashboardPage. Add a vehicle detail tab "Service" (the filtered list) and a tab "Reminders". Register the ⌘K action "New service entry" in QuickActions.

**Checkpoint**: quickstart V2 (except the PDF attachment) and V3 pass. Reminder states update right after any odometer or service write, and on the scheduler tick. `make check test` is green.

---

## Phase 5: Self-Repair Projects, Parts BOM & Attachments (US4, plus attachments for US1, US2 and US5)

**Goal**: The repair workspace (status workflow, diagnostics, fault codes, torque specs, tools, BOM with totals and status counts, board and list views, search), plus a shared attachment store wired into vehicle avatars, service logs, spec entries and repairs.

**Independent Test**: quickstart V4 (all steps), the V2 step 4 PDF receipt, and the V5 step 2 image. Create "Injector seal replacement" with P0201, 4 BOM parts, then torque specs and tools. Completing it without km is rejected. Complete it at 216,000 km (the odometer updates) and search for P0201. Upload a receipt, avatar and belt diagram. Deleting the owner removes the files.

### Tests

- [X] T080 [P] [US4] Write repair domain tests in internal/repairs/repairs_test.go:
  - Moving to `completed` without `completed_on` or `completion_odometer_km` returns 422.
  - Completing raises the vehicle odometer through a `repair_completion` reading.
  - Reopening deletes that reading but keeps the completion values.
  - `started_on` defaults to today on the first move to `in_progress`.
  - Fault codes are uppercased and deduplicated.
  - BOM totals are `Σ quantity × unit_cost_cents` with integer math, and status counts are correct.
- [X] T081 [P] [US4] Write attachment tests in internal/attachments/attachments_test.go:
  - Sniffing accepts JPEG, PNG, WebP, HEIC (ISO-BMFF `ftyp` brands `heic`, `heix`, `mif1`) and PDF.
  - It rejects HTML, SVG and ZIP with 415 `attachment.type_not_allowed`, even when the client claims `image/png`.
  - A file over the limit returns 413 `attachment.too_large` with `params.max_mb`.
  - Serving sets `nosniff`, `Content-Security-Policy: sandbox`, and `Content-Disposition: attachment` for HEIC.
  - Deleting the owner row removes the file after commit.
  - The orphan sweep deletes unreferenced files older than 1 h.
- [X] T082 [P] [US4] Write repair contract tests in internal/httpapi/repairs_test.go:
  - `POST /repairs`, `PATCH` (the `fault_codes` and `torque_specs` lists are replaced)
  - `POST /repairs/{id}/status`
  - part CRUD and reorder
  - `GET /repairs?status=planned&status=waiting_on_parts`
  - `q=P0201` matches through fault codes, and `q=<oem number>` matches through parts
  - `RepairSummary` includes `parts_total_cents` and `part_status_counts`

### Implementation

- [X] T083 [US4] Create migration internal/db/migrations/00005_repairs_attachments.sql, following data-model.md § 4 and § 5.

  **`repair_logs`**:
  - `vehicle_id … ON DELETE CASCADE`
  - `title TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 120)`
  - `status TEXT NOT NULL DEFAULT 'planned' CHECK (status IN ('planned','in_progress','waiting_on_parts','completed'))`
  - `symptom`, `root_cause`, `special_tools` as `TEXT NOT NULL DEFAULT ''`
  - `procedure_notes TEXT NOT NULL DEFAULT '' CHECK (length(procedure_notes) <= 50000)`
  - `started_on TEXT NULL`, `completed_on TEXT NULL`
  - `completion_odometer_km INTEGER NULL CHECK (completion_odometer_km >= 0)`
  - `CHECK (status <> 'completed' OR (completed_on IS NOT NULL AND completion_odometer_km IS NOT NULL))`
  - `CHECK (completed_on IS NULL OR started_on IS NULL OR completed_on >= started_on)`
  - `created_by`, timestamps
  - indexes `(vehicle_id, status)` and `(vehicle_id, completed_on)`

  **`repair_fault_codes`**:
  - `repair_log_id … ON DELETE CASCADE`
  - `code TEXT NOT NULL`
  - `UNIQUE (repair_log_id, code)`
  - index `(code)`

  **`repair_torque_specs`**:
  - `label TEXT NOT NULL`
  - `value_nm REAL NOT NULL CHECK (value_nm > 0)`
  - `note TEXT NOT NULL DEFAULT ''`
  - `sort_order INTEGER NOT NULL`

  **`repair_parts`**:
  - `name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 120)`
  - `oem_part_number`, `aftermarket_brand`, `aftermarket_part_number`, `supplier_url` as `TEXT NOT NULL DEFAULT ''`
  - `quantity INTEGER NOT NULL DEFAULT 1 CHECK (quantity >= 1)`
  - `unit_cost_cents INTEGER NULL CHECK (unit_cost_cents >= 0)`
  - `cost_type TEXT NOT NULL DEFAULT 'parts' CHECK (cost_type IN ('parts','fluids'))`
  - `status TEXT NOT NULL DEFAULT 'needed' CHECK (status IN ('needed','ordered','in_stock','installed'))`
  - `sort_order INTEGER NOT NULL`
  - indexes `(repair_log_id)`, `(oem_part_number)` and `(aftermarket_part_number)`

  **`attachments`**:
  - `storage_key TEXT NOT NULL UNIQUE`
  - `original_name TEXT NOT NULL CHECK (length(original_name) <= 255)`
  - `mime_type TEXT NOT NULL CHECK (mime_type IN ('image/jpeg','image/png','image/webp','image/heic','image/heif','application/pdf'))`
  - `size_bytes INTEGER NOT NULL`
  - `purpose TEXT NOT NULL DEFAULT 'file' CHECK (purpose IN ('file','avatar'))`
  - nullable owner FKs `vehicle_id`, `service_log_id`, `repair_log_id`, `spec_entry_id`, each `ON DELETE CASCADE`
  - `caption TEXT NOT NULL DEFAULT ''`
  - `uploaded_by`, `created_at`
  - `CHECK ((vehicle_id IS NOT NULL) + (service_log_id IS NOT NULL) + (repair_log_id IS NOT NULL) + (spec_entry_id IS NOT NULL) = 1)`
  - `CHECK ((purpose = 'avatar') = (vehicle_id IS NOT NULL))`
  - a partial unique index on `vehicle_id WHERE purpose = 'avatar'`

  Finally: `ALTER TABLE odometer_readings ADD COLUMN repair_log_id INTEGER NULL REFERENCES repair_logs(id) ON DELETE CASCADE`.
- [X] T084 [US4] Write sqlc queries:
  - internal/store/queries/repairs.sql: `CreateRepair`, `GetRepair`, `ListRepairs` (nargs for vehicle, statuses via `sqlc.slice`, q matching title/symptom/fault codes/part numbers via EXISTS subqueries), `CountRepairs`, `UpdateRepair`, `SetRepairStatus`, `DeleteRepair`, `ReplaceFaultCodes` (delete + insert), `ListFaultCodes`, `ReplaceTorqueSpecs`, `ListTorqueSpecs`, `CreatePart`, `UpdatePart`, `DeletePart`, `ListParts`, `SetPartOrder`, `RepairTotals` (`SUM(quantity*unit_cost_cents)` and the count per status), `UpsertRepairReading`, `DeleteRepairReading`, `ListOpenRepairs` (status ≠ completed, limit 10).
  - internal/store/queries/attachments.sql: `CreateAttachment`, `GetAttachment`, `ListAttachmentsFor{ServiceLog,Repair,SpecEntry}`, `GetVehicleAvatar`, `DeleteAttachment`, `UpdateAttachmentCaption`, `ListStorageKeys`.

  Then run `sqlc generate`.
- [X] T085 [US4] Implement the repair domain in internal/repairs/repairs.go:
  - CRUD with validation: title 1–120 chars, `procedure_notes ≤ 50000` chars, `completed_on ≥ started_on`.
  - Fault codes are trimmed and uppercased. OBD2 format `[PCBU][0-9A-F]{4}` is not enforced.
  - Torque specs need `value_nm > 0`.
  - `SetStatus(id, status, completedOn, completionKm)` follows the data-model § 4 side effects:
    - entering `completed` → `vehicles.RecordReading(source='repair_completion')` + `AfterOdometerChange`
    - leaving `completed` → `DeleteRepairReading`, keeping the completion values
    - the first move to `in_progress` sets `started_on` to today if empty
  - The `repairs` package must not import `servicelog` (Constitution II).
- [X] T086 [US4] Implement BOM operations in internal/repairs/parts.go:
  - Part CRUD with name 1–120 chars and `quantity ≥ 1`.
  - `supplier_url` must be empty or an absolute `http(s)` URL (`validation.url`).
  - `cost_type` defaults to `parts`, and `status` is one of `needed|ordered|in_stock|installed` with free transitions.
  - `Reorder(ids)` and `Totals(repairID)`.
- [X] T087 [US4] Implement the attachment store in internal/attachments/store.go:
  - `Save(ctx, r io.Reader, originalName, owner Owner, purpose)`:
    - streams the upload to a temp file under `<data>/attachments/tmp`, wrapped in `http.MaxBytesReader` at `TORQ_MAX_UPLOAD_MB`
    - sniffs the first 512 bytes (`http.DetectContentType`, plus an ISO-BMFF `ftyp` brand check for HEIC/HEIF)
    - enforces the allowlist; avatar and spec-entry uploads must be images
    - renames the file to `<data>/attachments/<yyyy>/<mm>/<32 hex random>`
    - inserts the row
  - `Open(id)` and `Delete(id)`. Files are removed only after the transaction commits.
  - `SweepOrphans(olderThan 1h)` compares the files on disk with `ListStorageKeys`.
  - Sanitize the original name: strip path separators and control characters, max 255 chars.
- [X] T088 [US4] Make vehicle, service log, repair and spec entry deletion collect the affected `storage_key`s before deleting and remove the files after commit. Do this in internal/vehicles/vehicles.go, internal/servicelog/servicelog.go, internal/repairs/repairs.go and internal/vehicles/specs.go. Add `attachments.SweepOrphans` to the scheduler's startup step in internal/scheduler/scheduler.go.
- [X] T089 [US4] Implement the attachment handlers in internal/httpapi/attachments.go:
  - `GET /attachments/{id}`, with `?download=1` forcing `attachment`. Responses set `nosniff` and `CSP: sandbox`. Disposition is `inline` for images and PDF, and `attachment` for HEIC/HEIF.
  - `PATCH /attachments/{id}` (caption) and `DELETE /attachments/{id}`.
  - Upload routes: `POST /service-logs/{id}/attachments`, `POST /repairs/{id}/attachments`, `POST /spec-entries/{id}/attachments`, `PUT /vehicles/{id}/avatar` (replaces any existing avatar) and `DELETE /vehicles/{id}/avatar`.
  - Fill in `avatar_url` in `VehicleSummary`. Include `attachments: [Attachment]` in the service log, repair and spec entry detail responses.
- [X] T090 [US4] Implement the repair handlers in internal/httpapi/repairs.go, following contracts/rest-api.md § Self-repair projects. Fill in `open_repairs` in internal/httpapi/dashboard.go from `ListOpenRepairs`.
- [X] T091 [P] [US4] Build the shared attachment UI:
  - web/src/components/FileDrop.tsx: drag-and-drop plus a file picker, with a client-side size pre-check against `max_upload_mb` from `/settings/instance` or a 25 MB default, and progress via `XMLHttpRequest`.
  - web/src/components/AttachmentGallery.tsx: image thumbnails using `loading="lazy"` and `object-fit`, a PDF/HEIC file tile, caption editing, delete with confirmation, and a lightbox for images.
- [X] T092 [US4] Build the repair workspace:
  - web/src/features/repairs/RepairBoardPage.tsx: four columns, Planned, In Progress, Waiting on Parts and Completed. Each card shows the title, vehicle, fault code chips and a "parts needed/ordered" count. Cards move with a status menu (no drag-and-drop library). A board/list toggle is remembered in `localStorage` inside a try/catch. The list view has status and vehicle filters plus search.
  - web/src/features/repairs/RepairDetailPage.tsx: sections for Diagnostics (symptom, fault code chips input, root cause), Procedure notes, Reference (torque specs table with label / Nm / note, and special tools as chips), Parts BOM, and Attachments.
  - web/src/features/repairs/CompleteRepairDialog.tsx: requires the completion date (default today) and the odometer (prefilled with the current km).
  - Add the `repairs` en/de keys.
- [X] T093 [US4] Build the BOM table web/src/features/repairs/PartsTable.tsx:
  - Inline-editable rows: name, OEM number, aftermarket brand and number, supplier URL (shown as a link with `rel="noopener noreferrer" target="_blank"`), quantity, unit cost, line total, cost type, and a status select with Badge colors.
  - Up/down reorder buttons.
  - A footer with the grand total and the count per status.
- [X] T094 [US4] Wire attachments into the existing screens:
  - an avatar upload on VehicleFormPage and avatar display on vehicle cards and the switcher
  - `FileDrop` + `AttachmentGallery` on ServiceLogDetailPage
  - image upload on SpecSheet custom entries (FR-015)
  - the gallery on RepairDetailPage

  Add the dashboard "Open repairs" panel in web/src/features/dashboard/OpenRepairs.tsx. Register the ⌘K action "New repair project". Add a vehicle detail tab "Repairs".

**Checkpoint**: quickstart V4 passes, along with the V2 receipt and V5 image steps. Uploads are type-checked by content. Deleting a vehicle, log or repair removes its files. `make check test` is green.

---

## Phase 6: Webhook Notification Engine (US6)

**Goal**: Admin-managed endpoints with masked custom headers, five payload presets with localized text, the transactional outbox enqueued on reminder transitions, a delivery worker with retries, a synchronous test send with response inspection, weekly/monthly digests, and the delivery log.

**Independent Test**: quickstart V6. Configure ntfy with an `Authorization` header and send a test (the status, body and duration are shown). The header value is never returned. Push a reminder into Due Soon and only one delivery appears in the log after several more evaluations. An unreachable endpoint retries and then shows `failed`, without affecting the others.

### Tests

- [X] T095 [P] [US6] Write golden-file tests for each preset in internal/notify/presets/presets_test.go:
  - Each preset renders `reminder_due_soon`, `reminder_overdue`, `digest_weekly` (empty and with 25 items), and `test`, in `en` and `de`.
  - Compare against internal/notify/presets/testdata/<preset>_<event>_<locale>.golden, with an `-update` flag to regenerate.
  - Discord: embeds limited to title 256, description 4096 and 25 fields, with "+N more".
  - ntfy: headers `Title` (RFC 2047 when non-ASCII), `Priority` 3/4/5, `Tags`, and `Click` only when a base URL is set.
  - Gotify: priority 2/5/8 and the `extras` structure.
  - Home Assistant and Generic JSON: the `schema: "torq.notification/v1"` body from contracts/webhook-payloads.md.
- [X] T096 [P] [US6] Write outbox and worker tests in internal/notify/notify_test.go, using an `httptest.Server`:
  - One transition enqueues one row per enabled, subscribed endpoint, in the same transaction.
  - Re-evaluating without a state change enqueues nothing.
  - Leaving to `upcoming` and re-entering `due_soon` enqueues again.
  - Paused reminders and archived vehicles enqueue nothing.
  - 2xx gives `succeeded`.
  - 500 and timeouts retry at +30 s, +2 min and +10 min (driven by a fake clock), then `failed` after 4 attempts.
  - 404 gives `failed` immediately. 429 and 408 retry.
  - Redirects are not followed.
  - `response_excerpt` is at most 2 KiB.
  - Retention keeps the newest 100 finished rows per webhook.
  - Custom header values are sent but never written to `request_headers_json`.
- [X] T097 [P] [US6] Write webhook contract tests in internal/httpapi/webhooks_test.go:
  - A non-admin gets 403.
  - Create and get show headers as `[{name, value_set: true}]`, never the value.
  - A `PATCH` header without `value` keeps the stored value, and a header left out is deleted.
  - The forbidden header names `Host`, `Content-Length`, `Transfer-Encoding` and `Connection` return 422.
  - A non-http(s) URL returns 422.
  - `POST /webhooks/{id}/test` returns `{delivery:{status,status_code,error,response_excerpt,duration_ms}}`.
  - `GET /settings/instance` and `PATCH` validate the digest ranges.

### Implementation

- [X] T098 [US6] Create migration internal/db/migrations/00006_webhooks.sql, following data-model.md § 7.

  **`webhooks`**:
  - `name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 64)`
  - `url TEXT NOT NULL CHECK (length(url) <= 2048)`
  - `preset TEXT NOT NULL CHECK (preset IN ('discord','ntfy','gotify','home_assistant','generic_json'))`
  - `enabled INTEGER NOT NULL DEFAULT 1`
  - `on_due_soon`, `on_overdue`, `on_weekly_digest`, `on_monthly_digest` as `INTEGER NOT NULL DEFAULT 0`
  - `owner_user_id INTEGER NULL REFERENCES users(id) ON DELETE SET NULL`
  - `last_weekly_digest_on TEXT NULL`, `last_monthly_digest_on TEXT NULL`
  - timestamps

  **`webhook_headers`**:
  - `webhook_id … ON DELETE CASCADE`
  - `name TEXT NOT NULL`
  - `value TEXT NOT NULL`
  - a unique index on `(webhook_id, name COLLATE NOCASE)`

  **`webhook_deliveries`**:
  - `webhook_id … ON DELETE CASCADE`
  - `event TEXT NOT NULL CHECK (event IN ('reminder_due_soon','reminder_overdue','digest_weekly','digest_monthly','test'))`
  - `reminder_id INTEGER NULL REFERENCES reminders(id) ON DELETE SET NULL`
  - `request_body BLOB NOT NULL`
  - `request_headers_json TEXT NOT NULL`
  - `status TEXT NOT NULL CHECK (status IN ('pending','succeeded','failed'))`
  - `attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count <= 4)`
  - `next_attempt_at TEXT NULL`
  - `last_status_code INTEGER NULL`
  - `last_error TEXT NOT NULL DEFAULT ''`
  - `response_excerpt TEXT NOT NULL DEFAULT ''`
  - `duration_ms INTEGER NULL`
  - `created_at`, `completed_at`
  - indexes `(status, next_attempt_at)` and `(webhook_id, created_at DESC)`
- [X] T099 [US6] Write sqlc queries in internal/store/queries/webhooks.sql:
  - `CreateWebhook`, `GetWebhook`, `ListWebhooks`, `UpdateWebhook`, `DeleteWebhook`
  - `ListWebhookHeaders`, `UpsertWebhookHeader`, `DeleteWebhookHeadersNotIn`
  - `ListSubscribedWebhooks` (by event flag, `enabled=1`)
  - `InsertDelivery`, `ClaimDueDeliveries` (`status='pending' AND next_attempt_at <= ?`, limit 20), `RecordAttempt`, `ListDeliveries`, `CountDeliveries`, `PruneDeliveries` (keep the newest 100 finished per webhook)
  - `SetDigestSent`, `ListDigestCandidates`

  Then run `sqlc generate`.
- [X] T100 [P] [US6] Implement backend i18n in internal/i18n/i18n.go, with internal/i18n/en.json and internal/i18n/de.json:
  - `T(locale, key, params)` with English fallback, and `FormatKm`, `FormatDate(date, dateFormat)` and `FormatNumber`, using `golang.org/x/text/message` with `language.German` or `language.English` chosen by `number_format` (research R16).
  - Message keys: notification titles and bodies for due soon, overdue, digest (including "Nothing due") and test, the built-in category names (matching the frontend `service:categories.*`), and field labels (Vehicle, Odometer).
- [X] T101 [US6] Implement the message model and presets in internal/notify/message.go and internal/notify/presets/{discord,ntfy,gotify,homeassistant,generic}.go:
  - `Message{Event, Severity, Title, Body, URL, Reminder, Vehicle, Digest, SentAt}` is built from DB rows plus the endpoint owner's preferences (English/ISO fallback when the owner is NULL).
  - `Render(preset, Message) (method, headers map[string]string, body []byte)` follows contracts/webhook-payloads.md exactly.
  - The `url` is omitted when `TORQ_BASE_URL` is empty.
- [X] T102 [US6] Implement the outbox in internal/notify/outbox.go:
  - `EnqueueTransitions(ctx, tx, transitions)` maps `due_soon` to `reminder_due_soon` and `overdue` to `reminder_overdue`. For each subscribed enabled webhook it renders and inserts a `pending` delivery with `next_attempt_at = now`.
  - Wire it as the `reminders.OnTransitions` hook.
  - After commit, call `Worker.Wake()` (non-blocking send on a buffered channel of size 1).
- [X] T103 [US6] Implement the delivery worker in internal/notify/worker.go:
  - One goroutine that wakes on the channel or a 30 s poll, then runs `ClaimDueDeliveries`.
  - Each delivery is sent with an `http.Client{Timeout: 10s, CheckRedirect: return http.ErrUseLastResponse}`, merging the preset headers with the custom headers loaded at send time and adding `User-Agent: torq/<version>`.
  - It records the status code, error class (`timeout`, `connection_refused`, `dns`, `tls`, `other`), the 2 KiB excerpt and the duration.
  - Retry policy (research R10): 2xx gives `succeeded`. 408, 429, 5xx and network errors retry at +30 s, +2 min and +10 min. Any other 4xx, or exhausted retries, gives `failed`.
  - Prune after each completion.
  - Start it from cmd/torq/main.go with graceful stop.
- [X] T104 [US6] Implement digests and the test send in internal/notify/digest.go and internal/notify/test.go:
  - `EnqueueDigests(ctx, now)` reads `instance_settings`. When local time is at or past the configured weekday or day and the hour, and `last_*_digest_on` is before the current period start, it builds one digest listing all overdue and due_soon reminders across non-archived vehicles (an empty list still sends "Nothing due"), enqueues it, and sets `last_*_digest_on`.
  - `SendTest(ctx, webhookID)` renders a `test` event from the first vehicle, or placeholder data if there is none, sends synchronously with the same client, records a delivery row, and returns the result.
  - Add `EnqueueDigests` and `PruneDeliveries` to the scheduler tick in internal/scheduler/scheduler.go.
- [X] T105 [US6] Implement the webhook and instance-settings handlers in internal/httpapi/webhooks.go and internal/httpapi/instance_settings.go, both admin only, following contracts/rest-api.md § Webhooks and § Instance settings:
  - Validation: name 1–64 chars, absolute http(s) URL ≤ 2048 chars, preset in the enum.
  - Header names must be RFC 7230 tokens and not `Host`, `Content-Length`, `Transfer-Encoding` or `Connection`.
  - `owner_user_id` is set to the creating admin.
  - Responses never include header values.
- [X] T106 [US6] Build the webhook UI:
  - web/src/features/settings/WebhooksPage.tsx: a list with preset icon, enabled toggle and events.
  - web/src/features/settings/WebhookForm.tsx: name, URL, a preset select with per-preset URL help text, event checkboxes, and a headers editor. Saved headers show "••••• (set)" with a "Replace" action.
  - web/src/features/settings/WebhookTestPanel.tsx: a "Send test notification" button showing the status code badge, duration, error class and a scrollable `<pre>` of the response excerpt.
  - web/src/features/settings/DeliveryLog.tsx: a paginated table of event, time, attempts, status and code, with an expandable excerpt.
  - web/src/features/settings/InstanceSettingsPage.tsx: digest weekday and hour, monthly day (1–28) and hour, plus read-only version, time zone, base URL configured, and max upload.
  - Add the `webhooks` and `settings` en/de keys.

**Checkpoint**: quickstart V6 passes. Exactly one delivery per transition. Deliveries survive a restart because they are persisted. `make check test` is green.

---

## Phase 7: Reports, Exports & Docker Packaging (US7, plus cross-cutting packaging)

**Goal**: The cost report (service vs. repair, parts vs. fluids vs. other, cost per km, per-vehicle breakdown), the vehicle timeline, the printable history and JSON/CSV export, and the production container with compose and healthcheck.

**Independent Test**: quickstart V7 and the container section. Seeded 2026 data shows €1,600 = €420 service + €1,180 repairs and €0.08/km. A one-reading range shows "not enough data". The exports respect `include_costs=false`. The print view comes out black on white on paper. `docker run` with only `/data` mounted becomes healthy.

### Tests

- [X] T107 [P] [US7] Write report tests in internal/reports/reports_test.go, with fixture data built through the domain functions:
  - Routine = Σ item `cost_cents` + Σ `extra_cost_cents`.
  - Self-repair counts only `status='completed'` with `completed_on` in range, as Σ `quantity × unit_cost_cents`.
  - Parts, fluids and other are split by `cost_type`, with extra cost as other.
  - `distance_km` = max − min reading in range, and `cost_per_km_cents` is null when there are fewer than 2 readings or the distance is 0.
  - Garage-wide totals equal the sum of the `by_vehicle` totals.
  - A reopened repair is excluded.
  - All results are exact to the cent (SC-007).
- [X] T108 [P] [US7] Write export tests in internal/reports/export_test.go:
  - JSON has `schema: "torq.vehicle-history/v1"` and contains every service item and every completed repair with its BOM, fault codes and torque specs, plus attachment metadata only.
  - CSV starts with the UTF-8 BOM, follows RFC 4180 quoting (a test with commas, quotes, newlines and umlauts), and has exactly the column header `record_type,date,odometer_km,record_title,category_or_part,brand,specification,quantity,unit,oem_part_number,aftermarket_part_number,cost,cost_type,status,notes`.
  - `include_costs=false` empties every cost value, and `include_notes=false` empties the notes and procedure text.
  - A 500-record export finishes in under 10 s (SC-008, as a benchmark-style test).

### Implementation

- [X] T109 [US7] Write sqlc queries in internal/store/queries/reports.sql:
  - `ServiceCostsByType` (vehicle narg, from, to)
  - `ServiceExtraCosts`
  - `RepairCostsByType` (completed only, `completed_on` range)
  - `ReadingRange` (min and max `odometer_km` plus count per vehicle in range)
  - `TimelineServices` and `TimelineRepairs` (merged in Go, or a single `UNION ALL` if sqlc parses it; otherwise use hand-written SQL in internal/reports with a comment, per the plan's Risks)
  - `HistoryServiceLogs`, `HistoryRepairs`

  Then run `sqlc generate`.
- [X] T110 [US7] Implement cost aggregation in internal/reports/costs.go. `CostReport(ctx, vehicleID *int64, from, to)` returns the `CostReport` shape from contracts/rest-api.md § Reports & export. `cost_per_km_cents` is computed as a float only at the end. Garage-wide totals sum the per-vehicle distances.
- [X] T111 [US7] Implement the timeline in internal/reports/timeline.go. `Timeline(ctx, vehicleID, from, to, limit, offset)` merges service logs (date = `service_date`) with completed repairs (date = `completed_on`), ordered date DESC then km DESC. Each item is `{kind, id, date, odometer_km, title_key|title, categories?, total_cents}`. It is read-only.
- [X] T112 [US7] Implement the history export in internal/reports/export.go:
  - `HistoryJSON(ctx, vehicleID, opts)` streams the versioned document described in research R19.
  - `HistoryCSV(ctx, vehicleID, opts, w)` writes the BOM, uses `encoding/csv`, emits one row per service item, per BOM line, and per repair without parts, and writes cost values as plain `89.50` with a dot.
  - Built-in category names are taken from the requester's locale via internal/i18n.
- [X] T113 [US7] Implement the report handlers in internal/httpapi/reports.go: `GET /reports/costs`, `GET /vehicles/{id}/timeline`, and `GET /vehicles/{id}/history?format=json|csv&include_costs&include_notes`. The history route sets `Content-Disposition: attachment; filename="torq-<make>-<model>-<YYYY-MM-DD>.<ext>"`, with make and model slugified to ASCII.
- [X] T114 [US7] Build the report page web/src/features/reports/ReportsPage.tsx:
  - A vehicle select (or "All vehicles") and a date range with presets: this year, last year, last 12 months, all time.
  - Total and cost-per-km tiles. Cost per km shows "not enough data" when null, formatted as `fmt.money(cents)/km`.
  - Two CSS bar meters: Service vs. Repair, and Parts vs. Fluids vs. Other. No chart library.
  - A per-vehicle table when garage-wide.
  - Add the `reports` en/de keys.
- [X] T115 [US7] Build the timeline web/src/features/reports/TimelinePage.tsx, as a vehicle detail tab "History": a vertical list with type icons (lucide `Wrench` for service, `Hammer` for repair), date, km, title or categories and total. Each entry links to its record. Pagination via "Load more".
- [X] T116 [US7] Build the printable history route web/src/features/reports/HistoryPrintPage.tsx at `/vehicles/:id/history/print`:
  - It renders outside the app Layout, with no navigation.
  - It shows the vehicle profile, the spec sheet summary, and the full service and repair history from the JSON endpoint, all through `useFormat()`.
  - Options panel (hidden when printing via `print:hidden`): include costs, include notes, a "Print" button calling `window.print()`, and "Download JSON" and "Download CSV" links to the export endpoints.
  - Paper-only light styles with `print:bg-white print:text-black`. The screen stays dark (research R15).
- [X] T117 Create the multi-stage Dockerfile at the repository root, following research R20:
  1. `node:22-alpine`: `npm ci` and `npm run build` in web/.
  2. `golang:1.23-alpine` or newer: copy web/dist, then `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/torq ./cmd/torq`.
  3. `gcr.io/distroless/static-debian12:nonroot`: copy the binary. `ENV TORQ_DATA_DIR=/data`, `VOLUME /data`, `EXPOSE 8080`, `USER nonroot`, `HEALTHCHECK CMD ["/torq","healthcheck"]`, `ENTRYPOINT ["/torq"]`.

  Add a .dockerignore covering node_modules, .data, bin and .git.
- [X] T118 [P] Create docker-compose.yml at the repository root:
  - Service `torq` built from `.` with image `torq:latest`.
  - Ports `8080:8080`, volume `torq-data:/data`.
  - Environment `TZ=Europe/Berlin` and a commented-out `TORQ_BASE_URL`.
  - A healthcheck using `["CMD","/torq","healthcheck"]` (interval 30 s, timeout 5 s, retries 3) and `restart: unless-stopped`.
  - Declare the named volume.
- [X] T119 Add the container check to .github/workflows/ci.yml:
  - `docker buildx build --platform linux/amd64,linux/arm64`.
  - Run the amd64 image with only `-v <tmp>:/data` and wait for a healthy status.
  - Assert that `/data/torq.db` exists and that `curl /healthz` returns ok.
  - Fail if the image is larger than 30 MB (Constitution III and the plan's constraints).

**Checkpoint**: quickstart V7 and the container section pass. The app ships as one image with one volume.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: End-to-end checks, localization completeness, a footprint check, and documentation.

- [X] T120 [P] Write the Playwright end-to-end suite against the built container, with web/playwright.config.ts using `baseURL` from `TORQ_E2E_URL`:
  - web/e2e/v1-first-boot.spec.ts
  - web/e2e/v2-service-reminder.spec.ts
  - web/e2e/v4-repair.spec.ts
  - web/e2e/v7-reports-export.spec.ts
  - web/e2e/v8-localization.spec.ts

  Each implements the matching quickstart.md scenario. Wire `make e2e` and a CI job that runs it against the image from T119.
- [X] T121 [P] Implement the dev-only seed command `torq seed-demo [--vehicles N] [--years Y]` in cmd/torq/seed.go. It is guarded by `TORQ_ENABLE_SEED=true` and generates realistic service logs, repairs, readings and reminders through the domain functions. It is used by quickstart V7 and V9.
- [X] T122 Complete the German translations across all namespaces under web/src/i18n/locales/de/ and in internal/i18n/de.json. Make the `i18n:check` key-parity check and the `i18next/no-literal-string` lint rule blocking in CI, and check that every API error code used in internal/httpapi has an `errors:*` key in both locales.
- [X] T123 [P] Run an accessibility and mobile pass over web/src/components/ and the feature pages:
  - All controls are reachable by keyboard and have visible focus.
  - Text contrast is at least WCAG AA on zinc-950/900.
  - At 360 px width the layout stacks and tables scroll horizontally in their own container.
  - The service log form works on a phone (SC-002).
- [X] T124 Run footprint verification (quickstart V9): `docker run --memory=256m --cpus=1` with `seed-demo --vehicles 10 --years 5`, then measure page load (< 2 s) and idle and active RSS (< 100 MB). Record the results in specs/001-torq-core-platform/quickstart.md § V9, and fix any hotspot (missing indexes, N+1 queries in list endpoints).
- [X] T125 [P] Write README.md covering what torq is (the DIY scope and non-goals), Quick start (docker compose), the env var table from research R20, the data volume and backup (copy `/data`; automatic pre-migration backups in `/data/backups`), reverse-proxy notes (`TORQ_BASE_URL`, `TORQ_TRUSTED_PROXIES`, `Secure` cookie), webhook preset setup per target, and Development (`make setup dev check test`).
- [X] T126 Run a security review pass:
  - session cookie flags behind HTTPS
  - CSRF header enforcement on every mutating route (add a router-level test that walks all registered non-GET routes)
  - attachment serving headers
  - header values never logged (scrub in RequestLog and the worker)
  - argon2 parameters
  - login throttle behind a proxy

  Fix any findings in the affected internal/ files.
- [X] T127 Run every quickstart.md scenario (V1–V9) against the release image. Tick off each one in specs/001-torq-core-platform/quickstart.md and file any failures as follow-up tasks.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Foundation)**: No dependencies. Blocks everything.
- **Phase 2 (Auth & Localization)**: Depends on Phase 1. Blocks every later phase, because every route requires a user and every UI uses i18n and the formatters.
- **Phase 3 (Garage & Specs)**: Depends on Phase 2.
- **Phase 4 (Service & Reminders)**: Depends on Phase 3, which provides vehicles and the `RecordReading`/`AfterOdometerChange` hook.
- **Phase 5 (Repairs & Attachments)**: Depends on Phase 3 (vehicles and odometer). Its attachment wiring into service logs (T094) depends on Phase 4. The repair work itself (T080, T082–T086, T090, T092–T093) could run in parallel with Phase 4.
- **Phase 6 (Webhooks)**: Depends on Phase 4, which provides reminder transitions and the scheduler.
- **Phase 7 (Reports & Packaging)**: Reports depend on Phases 4 and 5 (both cost sources). Packaging (T117–T119) depends only on Phase 1 and can be pulled forward at any time.
- **Phase 8 (Polish)**: Depends on all earlier phases.

### User Story Dependencies

| Story | Delivered in | Depends on |
|---|---|---|
| US1 | Phase 2 + Phase 3, with avatar in Phase 5 | – |
| US8 | Phase 2, completed in T122 | – |
| US5 | Phase 3, with images in Phase 5 and hints in Phase 4 | US1 |
| US2 | Phase 4, with receipts in Phase 5 | US1 |
| US3 | Phase 4 | US1 and US2 (linked reminders need service history) |
| US4 | Phase 5 | US1 |
| US6 | Phase 6 | US3 |
| US7 | Phase 7 | US2 and US4 |

### Within Each Phase

1. Write the tests and run them so they fail.
2. Add the migration, then the sqlc queries, then `sqlc generate`.
3. Write the domain package.
4. Add the HTTP handlers.
5. Build the UI.
6. Pass the checkpoint.

The `sqlc generate` step means query-file tasks within a phase run in sequence.

### Parallel Opportunities

- **Phase 1**: T003, T004, T005 and T007 in parallel after T001–T002. Then T009, T014 and T015, and T021–T024, in parallel once their targets exist.
- **Phase 2**: tests T028–T030 together. T033 (argon2) in parallel with T040–T042 (frontend i18n and formatters).
- **Phase 3**: tests T046–T047 together. The spec sheet backend (T052, T054) in parallel with the vehicle backend (T050, T053). T055 (UI components) in parallel with the backend work.
- **Phase 4**: tests T061–T063 together. The pure engine (T068) in parallel with the category and service log domain (T066–T067).
- **Phase 5**: tests T080–T082 together. T091 (attachment UI components) in parallel with the backend.
- **Phase 6**: tests T095–T097 together. T100 (backend i18n) in parallel with T098–T099.
- **Phase 7**: tests T107–T108 together. T118 (compose) in parallel with the report work.
- **Phase 8**: T120, T121, T123 and T125 in parallel.

---

## Parallel Example: Phase 4 (US2 + US3)

```bash
# Tests first, all independent files:
Task: "T061 reminder engine table tests in internal/reminders/engine/engine_test.go"
Task: "T062 transition tests in internal/reminders/evaluate_test.go"
Task: "T063 service log contract tests in internal/httpapi/service_logs_test.go"

# After T064/T065/T069 (migration + queries), build in parallel:
Task: "T068 pure engine in internal/reminders/engine/engine.go"
Task: "T066 categories domain in internal/servicelog/categories.go"
```

## Parallel Example: Phase 6 (US6)

```bash
Task: "T095 preset golden tests in internal/notify/presets/presets_test.go"
Task: "T096 outbox/worker tests in internal/notify/notify_test.go"
Task: "T097 webhook contract tests in internal/httpapi/webhooks_test.go"
Task: "T100 backend i18n catalogs in internal/i18n/"
```

---

## Implementation Strategy

### MVP First

The MVP is Phases 1–4. It delivers the P1 stories: US1 garage and auth, US2 service logbook, and US3 reminders.

1. Phase 1 → Phase 2 → Phase 3. Stop and check quickstart V1, which gives a usable garage.
2. Phase 4. Stop and check quickstart V2 and V3. This is a self-hosted service logbook with working reminders, the core value of torq.
3. Optionally pull T117–T118 (Dockerfile and compose) forward so the MVP can be deployed as a container right away.

### Incremental Delivery

4. **Phase 5**: repair projects and BOM (P2), plus attachments everywhere.
5. **Phase 6**: webhook notifications (P2). Reminders now reach the user's phone.
6. **Phase 7**: reports, export and production packaging (P3).
7. **Phase 8**: e2e tests, full German, footprint, docs, security review. This is the release candidate.

Each checkpoint leaves `main` buildable, green, and demo-able.

---

## Notes

- [P] marks tasks on different files with no dependency on unfinished tasks. sqlc generation and the shared router file (`internal/httpapi/router.go`, where each handler task registers its routes) are serialization points.
- Never edit a migration that has already been merged. Add a new forward migration instead (Constitution III).
- Domain packages `servicelog` and `repairs` must never import each other. Only `reports` reads both (Constitution II).
- Commit after each task or logical group, and stop at any checkpoint to validate.
