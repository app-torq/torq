# Quickstart & Validation Guide: torq Core Platform

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

This guide shows how to run torq and check the spec's main flows from start to finish. API shapes are in
[contracts/rest-api.md](./contracts/rest-api.md) and webhook formats in
[contracts/webhook-payloads.md](./contracts/webhook-payloads.md). The Playwright suite (`web/e2e/`) automates
the scenarios marked **[e2e]**.

## Prerequisites

| Tool | Version | Purpose |
|---|---|---|
| Go | 1.26+ | Backend |
| Node.js | 22 LTS (with npm) | Frontend build |
| sqlc | 1.27+ | Only needed after changing `internal/store/queries` |
| Docker (with buildx) | 24+ | Container build and the Constitution III check |
| A webhook catcher | e.g. `docker run -p 8090:80 binwiederhier/ntfy serve`, or any HTTP request bin on the LAN | Webhook scenarios |

## Run in development

```bash
make setup            # go mod download + npm ci --prefix web
make dev              # Go API on :8080 (TORQ_DATA_DIR=./.data) + Vite on :5173 (proxies /api)
# open http://localhost:5173
```

## Run as a container

```bash
docker build -t torq:dev .
docker run --rm -p 8080:8080 -v torq-data:/data -e TZ=Europe/Berlin torq:dev
curl -s localhost:8080/healthz      # → {"status":"ok","version":"…"}
```

Pass condition (Constitution III): the container starts with only `/data` mounted and no other services.
`/data` then contains `torq.db`, `torq.db-wal` and `attachments/`.

## Quality gates (CI runs these on every PR)

```bash
make check     # go vet, staticcheck, golangci-lint, sqlc diff check, tsc --noEmit, eslint, i18n key check
make test      # go test ./... (reminder engine, reports, presets golden files, API contract tests) + vitest
make e2e       # builds the image, runs it, runs the Playwright suite against it
```

## Validation scenarios

### V1: First boot & garage (US1) [e2e]

1. Open the app on an empty `/data`. You are redirected to setup. Create admin `marek` with a password of
   at least 10 characters.
2. Restart the container. Setup is not offered again (`GET /api/v1/setup/status` returns
   `setup_required: false`).
3. Add a vehicle: Mercedes-Benz / C 220 CDI / S203 / 2006 / OM646 / 214,500 km. The garage shows it with a
   placeholder avatar.
4. Dashboard quick-update to 215,200 km. The vehicle shows 215,200 km.
5. Quick-update to 21,520 km. A confirmation asks about lowering the odometer, and cancelling keeps
   215,200.
6. Change the password in a second browser session. The first session gets a 401 on its next request.

### V2: Service log resets a reminder (US2 + US3) [e2e]

1. Create a reminder: Engine Oil & Filter, every 10,000 km or 12 months, start date 3 months ago / 205,500 km (time limit stays far away).
2. Quick-update the odometer to 214,600 km. The reminder shows **Due Soon** with 900 km remaining.
3. Quick-update to 215,600 km. The reminder shows **Overdue** by 100 km.
4. Log a service on today's date at 215,600 km with three items: oil (Mobil 1, MB 229.51, 6.5 L, €89.50),
   air filter, cabin filter. Attach a PDF receipt.
5. The reminder shows **Upcoming**, next due at 225,600 km and today + 12 months.
6. Back-fill an oil service dated a year earlier at 200,000 km. The reminder is unchanged and the odometer
   stays at 215,600 km.
7. Delete the service from step 4. The reminder falls back to its start baseline and shows **Overdue**
   again.

### V3: Time-based reminder without user activity (US3, SC-004)

1. Create a "Brake Fluid every 24 months" reminder with start date today − 23 months − 5 days and a 30-day
   lead.
2. Restart the container, or wait for one scheduler tick (≤ 15 min). The reminder is **Due Soon**.
   Automated version: `go test ./internal/reminders/...` covers month clamping and composite triggers with
   a fixed clock.

### V4: Repair project & BOM (US4) [e2e]

1. Create the project "Injector seal replacement" with fault codes `P0201` and `p0300` (the second is
   stored as `P0300`) and status Planned.
2. Add 4 parts as Needed (seals, bolts, and so on) with unit costs. The totals and "4 needed" count appear.
3. Move 2 parts to Ordered, set the project to Waiting on Parts, and check the board column.
4. Add the torque spec "Injector clamp bolt 7 Nm, + 90°" and the tools "E12 Torx" and "Injector puller".
5. Set the project to Completed without an odometer. The save is rejected. Then complete it at 216,000 km.
   The vehicle odometer becomes 216,000 km and the project appears in the timeline.
6. Search `P0201` in the project list. The project is found.
7. Reopen the project as In Progress. It disappears from the timeline and the cost report.

### V5: Spec sheet (US5)

1. Fill in oil (5W-30, MB 229.51, 6.5 L), lug torque 150 Nm, and tire pressures 2.3 / 2.5 bar.
2. Add the custom entry "Serpentine belt routing" with an image, then reorder it to the top.
3. Start a new service log and pick Engine Oil & Filter. The "5W-30 · MB 229.51 · 6.5 L" hint appears.

### V6: Webhooks (US6, SC-005, SC-006)

1. Add an ntfy endpoint (topic URL on the local ntfy), events Due Soon + Overdue, and the custom header
   `Authorization: Bearer test`.
2. Send a test notification. The app shows status 200, the response body and the duration, and the ntfy
   client shows the message.
3. Reload the endpoint. The header shows as set, with no value displayed (`GET /webhooks/{id}` has no
   value).
4. Push a reminder into Due Soon, then run 3 more scheduler ticks or odometer updates without crossing
   another threshold. The delivery log shows exactly one `reminder_due_soon` delivery.
5. Point a second endpoint at an unreachable URL. The delivery log shows retry attempts and finally
   `failed`, while the first endpoint keeps working.
6. Repeat step 2 for the Discord, Gotify, Home Assistant and Generic JSON presets against the real services
   or a request bin. Compare the bodies with the golden files in `internal/notify/presets/testdata/`.

### V7: Reports & export (US7, SC-007, SC-008) [e2e]

1. Seed the data with `TORQ_ENABLE_SEED=true ./bin/torq seed-demo` (dev only; creates user `demo` /
   `torq-demo-password`): one vehicle, 2026 services totalling €420, completed repairs totalling €1,180, and
   readings of 205,000 → 225,000 km.
2. Open the 2026 report. It shows total €1,600, Service €420 / Repairs €1,180, and €0.08/km.
3. Choose a range with a single reading. Cost per km shows "not enough data".
4. Export JSON and CSV with costs turned off. Neither file contains cost values. The CSV opens in a
   spreadsheet with correct umlauts.
5. Open the printable view and print to PDF. It shows black text on white, no navigation, and the complete
   history.

### V8: Localization (US8, SC-009) [e2e]

1. Set German, `DD.MM.YYYY`, comma decimals and EUR. The service from V2 shows its date as `DD.MM.YYYY`,
   "215.600 km" and "89,50 €".
2. `make check` passes the i18n key check: no `de` key is missing and no literal JSX strings remain.
3. A webhook test sent by a German-locale admin produces German title and message text.

### V9: Footprint (SC-010)

On a Raspberry Pi 4 or a container limited with `--memory=256m --cpus=1`, after
`seed-demo --vehicles 10 --years 5`:
- dashboard and garage pages load in under 2 s
- RSS stays under 100 MB at idle and while browsing

**Measured 2026-10-31** (`docker run --memory=256m --cpus=1`, `seed-demo --vehicles 10 --years 5`: 10 vehicles,
191 service entries, 27 completed repairs): RSS 28 MB idle and 24–28 MB after 150 mixed API requests; every
endpoint (dashboard, vehicles, service log, repairs, reminders, cost report, timeline, history export) answered in
under 10 ms. Image size 24.7 MB.

## Validation log

| Scenario | Result (2026-10-31) | How |
|---|---|---|
| Container | ✅ starts healthy with only `/data` mounted; `torq.db` + WAL created | `docker run` + healthcheck |
| V1 first boot & garage | ✅ | Playwright `v1-first-boot` (binary and container) |
| V2 service log resets reminder | ✅ | Playwright `v2-service-reminder`; Go `reminders` and `httpapi` tests (back-fill, delete fallback) |
| V3 time-based reminder | ✅ | Go `reminders/engine` and `TestTimeBasedAdvancesWithClock` |
| V4 repair & BOM | ✅ | Playwright `v4-repair`; Go `repairs` tests |
| V5 spec sheet | ✅ | Go `TestSpecSheet`; spec hints and image upload exercised manually in the UI build |
| V6 webhooks | ✅ | Go `notify` tests against a local HTTP sink (exactly-once, retries, 4xx, redirects, unreachable, digests, retention); preset golden files |
| V7 reports & export | ✅ €1,600 = €420 + €1,180, €0.08/km, single reading → "not enough data" | Playwright `v7-reports-export`; seeded instance via API; Go `reports` tests |
| V8 localization | ✅ | Playwright `v8-localization`; `i18n:check` (586 keys, 57 API error codes) |
| V9 footprint | ✅ see measurements above | Docker with resource limits |
| Mobile / a11y | ✅ no horizontal scroll at 360 px on 10 pages; visible focus | Playwright `v9-mobile-a11y` |

Not verified here: delivery to real Discord, Gotify and Home Assistant servers (payloads are covered by golden
files and contract tests, not by live services).
