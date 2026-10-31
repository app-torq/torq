# Contract: torq REST API v1

**Feature**: [spec.md](../spec.md) | **Data model**: [data-model.md](../data-model.md)

The browser SPA is the only first-party client. This contract is also the basis for the Go HTTP contract tests.

## Conventions

- **Base path**: `/api/v1`. JSON request and response bodies (`Content-Type: application/json; charset=utf-8`), except uploads (`multipart/form-data`) and exports.
- **Auth**: the `torq_session` cookie (research R6). Every endpoint requires a session unless marked **public**. Endpoints marked **admin** require `role = admin`.
- **CSRF**: every non-`GET`/`HEAD` request must send `X-Torq-Request: 1`. A cross-origin `Origin` header is rejected with `403 csrf.origin_mismatch`.
- **Naming**: fields use `snake_case`. IDs are integers.
- **Types**: dates are `"YYYY-MM-DD"` strings and timestamps are RFC 3339 UTC strings.
- **Units**: money is an integer `*_cents` field. Odometer values are `*_km` integers, fluid quantities and capacities are `*_ml` integers, torque is `*_nm` (number), and pressure is `*_bar` (number).
- **Errors**: the body is always `{"error": {"code": "<dotted.code>", "fields": {"<field>": "<dotted.code>"}?, "params": {…}?}}`. The frontend translates every code (Constitution V). The API never sends human-readable prose.

| HTTP | When | Example code |
|---|---|---|
| 400 | Malformed JSON or bad query parameter | `request.malformed` |
| 401 | No session, or the session expired | `auth.unauthenticated` |
| 403 | Not an admin, or a CSRF failure | `auth.forbidden` |
| 404 | Unknown ID | `not_found` |
| 409 | Business rule conflict | `user.last_admin`, `category.in_use`, `odometer.decrease_requires_confirmation` |
| 413 | Upload too large | `attachment.too_large` (`params.max_mb`) |
| 415 | File type not allowed | `attachment.type_not_allowed` |
| 422 | Field validation failed | `validation.failed` with `fields` |
| 429 | Login throttled | `auth.throttled` (`params.retry_after_s`) |

- **Warnings**: a write can succeed and still return `"warnings": ["odometer.inconsistent_with_history", …]` next to the resource.
- **Lists**: list endpoints return `{"items": […], "total": n}` and take `limit` (default 50, max 200) and `offset`.
- **Re-evaluated reminders**: any write that changes an odometer reading or the service history returns the vehicle's re-evaluated reminder summary as `"reminders": [ReminderSummary]`, so the client can update its cache without another request.

---

## Setup & Auth

| Method & Path | Access | Request | Response |
|---|---|---|---|
| `GET /setup/status` | public | – | `{"setup_required": bool}` |
| `POST /setup` | public, only while `setup_required` is true (otherwise `409 setup.already_completed`) | `{username, display_name, password, locale?}` | `201 {user, instance}` and sets the session cookie |
| `POST /auth/login` | public | `{username, password}` | `200 {user, instance}` and sets the cookie. Wrong credentials return `401 auth.invalid_credentials`, the same response for an unknown user and a wrong password |
| `POST /auth/logout` | session | – | `204` and clears the cookie |
| `GET /auth/me` | session | – | `{user, instance: {max_upload_mb, version}}` |
| `POST /auth/password` | session | `{current_password, new_password}` (new password ≥ 10 chars) | `204`. Ends all other sessions |
| `PATCH /me/preferences` | session | `{display_name?, locale?, date_format?, number_format?, currency_code?, currency_symbol?}` | `{user}` |

`User` = `{id, username, display_name, role, locale, date_format, number_format, currency_code, currency_symbol, disabled: bool}`

## Users

| Method & Path | Access | Request | Response |
|---|---|---|---|
| `GET /users` | admin | – | `{items: [User]}` |
| `POST /users` | admin | `{username, display_name, password, role}` | `201 {user}` |
| `PATCH /users/{id}` | admin | `{display_name?, role?, disabled?}` | `{user}`. Returns `409 user.last_admin` when the change would leave no enabled admin |
| `POST /users/{id}/password` | admin | `{new_password}` | `204`. Ends all of that user's sessions |
| `DELETE /users/{id}` | admin | – | `204`. Returns `409 user.last_admin` for the last enabled admin and `409 user.cannot_delete_self` for your own account |

## Instance settings

| Method & Path | Access | Request / Response |
|---|---|---|
| `GET /settings/instance` | admin | `{weekly_digest_weekday, weekly_digest_hour, monthly_digest_day, monthly_digest_hour, base_url_configured: bool, time_zone, max_upload_mb, version}` |
| `PATCH /settings/instance` | admin | Any subset of the four digest fields |

## Vehicles & Odometer

| Method & Path | Request | Response |
|---|---|---|
| `GET /vehicles?archived=false\|true\|all` | – | `{items: [VehicleSummary]}` |
| `POST /vehicles` | `{make, model, chassis_code?, model_year?, engine_code?, vin?, license_plate?, current_odometer_km}` | `201 {vehicle}`. Also creates the empty spec sheet and the `initial` odometer reading |
| `GET /vehicles/{id}` | – | `{vehicle}` |
| `PATCH /vehicles/{id}` | Any profile field except `current_odometer_km` | `{vehicle}` |
| `DELETE /vehicles/{id}` | `{confirm_name: "<make> <model>"}` | `204`. A name that doesn't match returns `422 vehicle.confirm_name_mismatch` |
| `POST /vehicles/{id}/archive` and `/unarchive` | – | `{vehicle}`. Re-evaluates the vehicle's reminders |
| `POST /vehicles/{id}/odometer` | `{odometer_km, reading_date?, confirm_decrease?, confirm_large_jump?}` | `{vehicle, reminders: [ReminderSummary]}`. Returns `409 odometer.decrease_requires_confirmation` or `409 odometer.large_jump_requires_confirmation` (`params.current_km`) |
| `GET /vehicles/{id}/odometer-readings?from&to` | – | `{items: [{id, reading_date, odometer_km, source}]}` |
| `PUT /vehicles/{id}/avatar` | multipart `file` (image) | `{vehicle}` |
| `DELETE /vehicles/{id}/avatar` | – | `204` |

`VehicleSummary` = `{id, make, model, chassis_code, model_year, engine_code, license_plate, current_odometer_km, avatar_url|null, archived, reminder_counts: {due_soon, overdue}}`
`Vehicle` = `VehicleSummary` plus `{vin, created_at, updated_at}`

## Spec sheet

| Method & Path | Request | Response |
|---|---|---|
| `GET /vehicles/{id}/specs` | – | `{specs: VehicleSpecs, entries: [SpecEntry]}` |
| `PUT /vehicles/{id}/specs` | The full `VehicleSpecs` object (every field in data-model §2, nullable) | `{specs}` |
| `POST /vehicles/{id}/spec-entries` | `{label, value}` | `201 {entry}` |
| `PATCH /spec-entries/{id}` | `{label?, value?}` | `{entry}` |
| `DELETE /spec-entries/{id}` | – | `204` |
| `PUT /vehicles/{id}/spec-entries/order` | `{ids: [int]}` (must list every entry exactly once) | `{entries}` |
| `POST /spec-entries/{id}/attachments` | multipart `file` (image) | `201 {attachment}` |

`SpecEntry` = `{id, label, value, sort_order, attachments: [Attachment]}`

## Service categories

| Method & Path | Request | Response |
|---|---|---|
| `GET /service-categories?include_archived=false` | – | `{items: [{id, builtin_key\|null, custom_name\|null, is_fluid, default_interval_km, default_interval_months, archived}]}`. The client shows `t('service.categories.'+builtin_key)` or `custom_name` |
| `POST /service-categories` | `{custom_name, is_fluid, default_interval_km?, default_interval_months?}` | `201` |
| `PATCH /service-categories/{id}` | Custom categories: any field. Built-in categories: only the default intervals | `200` |
| `POST /service-categories/{id}/archive` and `/unarchive` | Custom categories only | `200` |
| `DELETE /service-categories/{id}` | Custom categories only | `204`, or `409 category.in_use` |

## Service logbook

| Method & Path | Request | Response |
|---|---|---|
| `GET /service-logs?vehicle_id&category_id&from&to&q&limit&offset` | `q` matches notes, brand, specification and part numbers | `{items: [ServiceLogSummary], total}` |
| `POST /service-logs` | `ServiceLogInput` | `201 {service_log, reminders, warnings?}` |
| `GET /service-logs/{id}` | – | `{service_log}` (includes items and attachments) |
| `PUT /service-logs/{id}` | `ServiceLogInput` (full replacement, items included) | `{service_log, reminders, warnings?}` |
| `DELETE /service-logs/{id}` | – | `{reminders}`. Linked reminders fall back to the previous baseline |
| `POST /service-logs/{id}/attachments` | multipart `file`, `caption?` | `201 {attachment}` |

```jsonc
// ServiceLogInput
{
  "vehicle_id": 1,
  "service_date": "2026-10-15",
  "odometer_km": 215000,
  "notes": "",
  "extra_cost_cents": 0,
  "extra_cost_label": "",
  "items": [
    {
      "category_id": 1,
      "brand": "Mobil 1",
      "specification": "MB 229.51",
      "quantity_ml": 6500,
      "quantity_unit": "L",
      "oem_part_number": "A6511800109",
      "aftermarket_part_number": "",
      "cost_cents": 8950,
      "cost_type": "fluids" // optional; defaults from the category
    }
  ]
}
```

## Self-repair projects

| Method & Path | Request | Response |
|---|---|---|
| `GET /repairs?vehicle_id&status&q&limit&offset` | `status` may repeat. `q` matches title, symptom, fault codes and part numbers | `{items: [RepairSummary], total}`. `RepairSummary` includes `{parts_total_cents, part_status_counts}` |
| `POST /repairs` | `{vehicle_id, title, status?, symptom?, root_cause?, procedure_notes?, special_tools?, started_on?, fault_codes?: [str], torque_specs?: [{label, value_nm, note?}]}` | `201 {repair}` |
| `GET /repairs/{id}` | – | `{repair}` (includes fault codes, torque specs, parts, attachments and totals) |
| `PATCH /repairs/{id}` | Any field except `status`. `fault_codes` and `torque_specs` replace the whole list when present | `{repair}` |
| `POST /repairs/{id}/status` | `{status, completed_on?, completion_odometer_km?}` | `{repair, vehicle?, reminders?}`. Completing without the date or odometer returns `422` |
| `DELETE /repairs/{id}` | – | `204` |
| `POST /repairs/{id}/parts` | `{name, oem_part_number?, aftermarket_brand?, aftermarket_part_number?, supplier_url?, quantity?, unit_cost_cents?, cost_type?, status?}` | `201 {part}` |
| `PATCH /repair-parts/{id}` | Any part field | `{part}` |
| `DELETE /repair-parts/{id}` | – | `204` |
| `PUT /repairs/{id}/parts/order` | `{ids: [int]}` | `{parts}` |
| `POST /repairs/{id}/attachments` | multipart `file`, `caption?` | `201 {attachment}` |

## Attachments

| Method & Path | Response |
|---|---|
| `GET /attachments/{id}?download=1` | The file bytes with the stored MIME type, `nosniff`, `CSP: sandbox`, and `Content-Disposition` as described in research R18 |
| `PATCH /attachments/{id}` | `{caption}` → `{attachment}` |
| `DELETE /attachments/{id}` | `204` |

`Attachment` = `{id, original_name, mime_type, size_bytes, caption, url, created_at}`

## Reminders & dashboard

| Method & Path | Request | Response |
|---|---|---|
| `GET /reminders?vehicle_id&state&include_paused=false` | – | `{items: [Reminder]}`, most urgent first |
| `POST /reminders` | `{vehicle_id, service_category_id?, title?, interval_km?, interval_months?, interval_days?, lead_km?, lead_days?, start_date?, start_odometer_km?}` | `201 {reminder}`. Validation codes: `reminder.interval_required`, `reminder.lead_exceeds_interval`, `reminder.title_required`, `reminder.months_and_days_exclusive` |
| `GET /reminders/{id}` | – | `{reminder}` |
| `PATCH /reminders/{id}` | Same fields as `POST` | `{reminder}` (re-evaluated) |
| `POST /reminders/{id}/done` | `{done_on?, odometer_km?}`. Standalone reminders only (linked ones return `409 reminder.linked_use_service_log`) | `{reminder, vehicle}` |
| `POST /reminders/{id}/pause` and `/resume` | – | `{reminder}` |
| `DELETE /reminders/{id}` | – | `204` |
| `GET /dashboard` | – | `{vehicles: [VehicleSummary], attention: [Reminder] (overdue then due soon), open_repairs: [RepairSummary] (not completed, max 10)}` |

```jsonc
// Reminder (ReminderSummary is the same object without the interval and lead fields)
{
  "id": 7,
  "vehicle_id": 1,
  "service_category_id": 1,
  "title": "",
  "interval_km": 10000,
  "interval_months": 12,
  "interval_days": null,
  "lead_km": 1000,
  "lead_days": 30,
  "baseline": {"date": "2026-10-15", "odometer_km": 215000, "source": "service_log"}, // source: service_log | mark_done | start
  "next_due_km": 225000,
  "next_due_date": "2027-10-15",
  "remaining_km": 9800,
  "remaining_days": 350,          // negative values mean overdue
  "state": "upcoming",            // upcoming | due_soon | overdue
  "state_trigger": null,          // km | time | null
  "paused": false
}
```

## Webhooks (admin)

| Method & Path | Request | Response |
|---|---|---|
| `GET /webhooks` | – | `{items: [Webhook]}` |
| `POST /webhooks` | `{name, url, preset, enabled?, events: {due_soon, overdue, weekly_digest, monthly_digest}, headers?: [{name, value}]}` | `201 {webhook}` |
| `GET /webhooks/{id}` | – | `{webhook}` |
| `PATCH /webhooks/{id}` | The same fields. `headers` is a full list of `{name, value?}`: a missing `value` keeps the stored one, and a header left out of the list is deleted | `{webhook}` |
| `DELETE /webhooks/{id}` | – | `204` |
| `POST /webhooks/{id}/test` | – | `{delivery: {status, status_code\|null, error\|null, response_excerpt, duration_ms}}`. Synchronous, 10 s timeout |
| `GET /webhooks/{id}/deliveries?limit&offset` | – | `{items: [{id, event, reminder_id, status, attempt_count, last_status_code, last_error, response_excerpt, duration_ms, created_at, completed_at}], total}` |

`Webhook` = `{id, name, url, preset, enabled, events, headers: [{name, value_set: true}], owner_user_id, created_at, updated_at}`. Header values are never included in a response.

## Reports & export

| Method & Path | Response |
|---|---|
| `GET /reports/costs?vehicle_id?&from&to` | See `CostReport` below. Leaving out `vehicle_id` gives the whole garage plus a `by_vehicle` breakdown |
| `GET /vehicles/{id}/timeline?from&to&limit&offset` | `{items: [{kind: "service"\|"repair", id, date, odometer_km, title_key\|title, categories?, total_cents}], total}` |
| `GET /vehicles/{id}/history?format=json\|csv&include_costs=true&include_notes=true` | JSON returns `application/json` with the `torq.vehicle-history/v1` document. CSV returns `text/csv; charset=utf-8` with a BOM. Both set `Content-Disposition: attachment; filename="torq-<make>-<model>-<date>.<ext>"` (research R19) |

```jsonc
// CostReport
{
  "from": "2026-01-01",
  "to": "2026-12-31",
  "total_cents": 160000,
  "by_source": {"routine_service_cents": 42000, "self_repair_cents": 118000},
  "by_cost_type": {"parts_cents": 121000, "fluids_cents": 37000, "other_cents": 2000},
  "distance_km": 20000,              // null if fewer than 2 readings or zero distance
  "cost_per_km_cents": 8.0,          // fractional cents; null when distance_km is null
  "by_vehicle": [ { "vehicle_id": 1, "...": "same fields" } ] // garage-wide only
}
```

## Health

| Method & Path | Access | Response |
|---|---|---|
| `GET /healthz` (at the root, not under `/api/v1`) | public | `200 {"status": "ok", "version": "…"}` once migrations are applied and the database answers `SELECT 1`. Otherwise `503` |
