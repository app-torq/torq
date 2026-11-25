# The torq API

torq's web app and the torq mobile app both use the same REST API under `/api/v1`. This page explains how to
sign in, how a client checks a server before signing in, and which changes the API promises not to make.

The contract itself is [`openapi.yaml`](../openapi.yaml) (OpenAPI 3.1) in the repository root. Every release
also has it attached, so a client can pin the exact contract of a server version. Tests in
`internal/httpapi` fail whenever the contract and the server disagree: about which operations exist, who may
call them, or what a response looks like.

## Signing in

| Client | Sign in with | Then send | Extra header on non-GET requests |
|---|---|---|---|
| Browser | `POST /api/v1/auth/login` | the `torq_session` cookie | `X-Torq-Request: 1` |
| App or script | `POST /api/v1/auth/login` with `"token": true` | `Authorization: Bearer <token>` | none |

A token sign-in looks like this:

```sh
curl -s -X POST https://torq.example.com/api/v1/auth/login \
  -H 'Content-Type: application/json' -H 'X-Torq-Request: 1' \
  -d '{"username": "marek", "password": "…", "token": true, "device_name": "Marek'\''s iPhone"}'
```

The response contains `user`, `instance` and `token`, and sets no cookie. `POST /api/v1/setup` accepts the
same two fields, so an app can also create the first account on a fresh server.

- Tokens are ordinary sessions: they expire after 30 days without use, and every use extends them.
- `POST /api/v1/auth/logout` with a token revokes that token only.
- Settings → Devices in the web app lists every signed-in browser and app and can sign them out
  (`GET`/`DELETE /api/v1/me/sessions`). Changing the password signs out everything else, tokens included.
- When a request carries a bearer token, only the token counts. An invalid token is rejected even if the
  request also has a valid cookie.
- `Authorization` headers with other schemes, such as basic auth added by a reverse proxy in front of torq,
  are ignored, and the cookie works as usual. A proxy must not forward a *bearer* token of its own to torq,
  or browser sign-ins break.

### Why bearer requests skip the CSRF header

Browsers attach cookies to requests from other sites on their own, so cookie requests must prove they come
from the torq web app. That is what `X-Torq-Request: 1` and the `Origin` check do. Browsers never add an
`Authorization` header to a cross-site request unless the server allows it through CORS, and torq sends no
CORS headers. **Keep it that way**: adding CORS headers to torq, or in a proxy in front of it, would make the
bearer exemption unsafe.

## Checking a server

`GET /api/v1/instance` needs no sign-in. Clients call it when a user enters a server address:

```json
{
  "api_version": 1,
  "version": "v1.1.0",
  "setup_required": false,
  "demo_mode": false,
  "token_auth": true,
  "spec_autofill": true,
  "max_upload_mb": 25
}
```

- `api_version` is the API level. Compare it to decide which features to offer (`api_version >= 2`, …).
- `version` is the build's version string, for display only. It can be a tag or a commit hash, so never
  parse it to detect features.
- Servers older than torq 1.1.0 have no `/instance` and answer `404`. Treat that as level 0: those servers
  have no token sign-in, so an app can't use them.
- `demo_mode` and `max_upload_mb: 0` tell a client to hide account changes and uploads.
- `branding` (API level 7) holds the instance name an admin set (`name`, empty when unset) and `logo_url`
  (`null` without a logo). Show them next to or instead of the torq logo. `logo_url` is versioned, so a
  cached logo can be kept until the URL changes. The logo can be PNG, JPEG or SVG; display it as an image
  only. In `GET /auth/me`, `branding` also has `contact` (address, phone, email, website) for printed or
  shared documents.
- `public_links` (API level 9) is `false` when an admin turned public vehicle links off. Hide the share
  controls then.

## Compatibility

App versions and self-hosted servers update independently, so within `/api/v1` the API only grows:

- No operation, field or error code is removed or renamed.
- No request field becomes required, and no field changes its meaning.
- New operations, fields, error codes and values are allowed.
- `api_version` rises by one with every release that adds something, and `info.version` in `openapi.yaml`
  always matches it.

Clients in turn must ignore fields they don't know, and handle unknown values in fields whose description
lists the possible values (`status`, `fuel_type`, …), since those lists can grow.

A change that breaks these rules needs a new `/api/v2`.

## Units

The API is metric, whatever the user's preferences: distances in km (`*_km`), volumes in mL (`*_ml`),
torque in Nm, pressure in bar, displacement in cm³ and power in kW (`power_kw`, whole kW) and W
(`power_w`, the precise value). Convert only for display and input:

- A vehicle's distances (odometer, service and repair entries, reminder intervals) use its
  `odometer_unit` (`km` or `mi`), since that is what its odometer shows.
- A vehicle's displacement and power use its `displacement_unit` and `power_unit` when they are set
  (API level 4); `null` means the user's preference.
- Everything else uses the signed-in user's preferences from `GET /auth/me`: `distance_unit`, `volume_unit`,
  `torque_unit`, `pressure_unit`, `displacement_unit` and `power_unit`, plus `date_format` and `time_format`.
- A service item's `quantity_unit` is the unit the amount was typed in (`L`, `mL`, `qt`, `gal`, `fl_oz`);
  `quantity_ml` is always the amount.

Integer storage loses nothing for whole miles, cubic inches or horsepower (sent as `power_w`). Still, send
an unchanged stored value back as it was rather than converting it twice.

### Hour meters (API level 6)

Operating hours are sent as numbers in hours with at most one decimal (`1250.4`) and are never converted.

- A vehicle lists its hour meters in `hour_meters` (at most one for now; treat it as a list). Service
  entries, repair completions and reminder completions carry hours in `meter_readings` /
  `completion_meter_readings`, keyed by `hour_meter_id`.
- `has_odometer: false` marks a vehicle tracked by hours only. Its `current_odometer_km` stays an integer
  (0) so older clients keep working, but it means nothing; show the hours instead. Likewise,
  `odometer_recorded` / `completion_odometer_recorded` say whether an entry's `odometer_km` /
  `completion_odometer_km` was recorded.
- Reminders can have an hours interval (`hour_meter_id`, `interval_hours`, `lead_hours`); `state_trigger`
  can then be `hours`. A `PATCH /reminders/{id}` without `hour_meter_id` keeps the hours settings.

### Vehicle types (API level 8)

- `vehicle_type` on vehicles is `car` or `motorcycle`; more types may follow, so treat unknown values like
  `car`. It defaults to `car` on create, and a `PATCH` without it keeps the stored type.
- The catalog cascade takes `vehicle_type` (`car` when omitted, so older clients see what they always saw).
  Motorcycles have no generations: call `GET /catalog/engines?vehicle_type=motorcycle&make=…&model=…`
  without `generation` to list a model's versions, each with its displacement, power and years of first
  registration (`year_from`, `year_to`). `GET /catalog/kba` is not filtered.
- The spec sheet has a nested `motorcycle` object (final drive, sprockets, chain, fork oil per fork leg in
  mL, valve clearance ranges in mm). It is stored for every vehicle type. **`PUT /vehicles/{id}/specs`
  without `motorcycle` keeps the stored motorcycle values**; send the object only when you show those
  fields, and then send all of them.
- Service categories carry `vehicle_type` (`car`, `motorcycle`, or `null` for all types), and
  `GET /service-categories?vehicle_type=…` lists those that apply to a type. The list of `builtin_key`
  values grows with new built-ins (`chain_maintenance`, `chain_sprockets`, `valve_clearance`, `fork_oil`,
  `final_drive_oil`, `drive_belt`); show an unknown key through your generic fallback.

### Share links (API level 9)

A vehicle can have one public, read-only link that opens without an account, e.g. for a workshop or a
buyer. Users with the `shares.manage` permission on the vehicle manage it; the controls belong next to the
vehicle's QR code.

- `GET /vehicles/{id}/share` returns `{"share": null}` or the link. `PUT /vehicles/{id}/share` turns it on
  (`201`) or changes its options (`200`): `include_costs`, `include_license_plate` and `include_vin`, all
  off for a new link; omitted options keep their value. The token stays the same when options change.
- `POST /vehicles/{id}/share/renew` issues a new token and retires the old one at once.
  `DELETE /vehicles/{id}/share` turns the link off; turning it on again creates a new token.
- `share.path` is `/s/<token>`. Prefix it with the server address the user entered to get the link to show,
  copy or encode in a QR code.
- `GET /shared/{token}` is the public record behind the link and needs no sign-in: vehicle profile,
  readings, service history, reminders, spec sheet, the instance branding and the instance's default
  `formats` for displaying values to a visitor. It never contains notes, attachments, repairs, user names
  or IDs; `license_plate`, `vin` and the cost fields are `null` unless the owner included them. Signed-in
  requests also get `vehicle_id`, to open the vehicle in the app. Category names follow `Accept-Language`
  (`de` or `en`).
- An unknown, renewed, turned-off or paused link (the vehicle is archived, or an admin turned public links
  off) always answers the same `404 not_found`. After 10 failed lookups from one address, further ones get
  `429 auth.throttled` with `retry_after_s`. Responses carry `Cache-Control: no-store`,
  `X-Robots-Tag: noindex, nofollow, noarchive` and `Referrer-Policy: no-referrer`.
- `PUT` and `renew` return `409 share.disabled` while public links are off, and `demo.shares_locked` in the
  demo. Admins list all links with `GET /shares` (no tokens) and switch the feature with
  `public_links_enabled` in `PATCH /settings/instance`.

### Roles and permissions (API level 10)

Every user has one role: a set of permissions plus whether its users see every vehicle or only the ones
assigned to them. The built-in roles are `admin` (Administrator, everything), `member` (every permission
but `catalog.update`, all vehicles; what every member could do before level 10) and `viewer` (read only,
all vehicles); administrators add their own.

- **What the signed-in user may do:** `GET /auth/me` (and the login and setup responses) include
  `user.permissions` and `user.all_vehicles`. Every vehicle in a response carries `permissions`: the
  per-vehicle permissions the user holds on that vehicle, empty when they may only read it. Hide the actions
  a user can't use; the server checks every request anyway.
- **Permissions:** per vehicle `vehicles.edit`, `vehicles.delete`, `specs.edit`, `service_logs.write`,
  `service_logs.delete`, `reminders.write`, `repairs.write`, `repairs.delete`, `shares.manage`,
  `history.export`; garage-wide `vehicles.create`, `categories.manage`, `repair_lists.manage`,
  `catalog.update`. The list can grow: treat unknown keys as "not granted" and look up new ones in
  `GET /roles`. Each operation in `openapi.yaml` names the permission it needs in `x-torq-permission`;
  operations about a vehicle's data without one only need the vehicle to be visible. Attachments follow the
  record they belong to.
- **Hidden vehicles:** a vehicle the user can't see, and every record belonging to it, answers
  `404 not_found` exactly like a record that doesn't exist. Lists, the dashboard, the cost report, tag lists
  and priority counts only cover visible vehicles; a `vehicle_id` filter for a hidden vehicle returns an
  empty list.
- **Missing permission:** a visible vehicle answers `403 auth.forbidden` for a change the role doesn't allow.
- **Older clients:** `role` stays `admin` (Administrator role) or `member` (every other role), and creating or
  changing a user with `role: "admin"` or `"member"` still works. New clients send `role_id`.
- **Administration** (Administrator only): `GET/POST /roles`, `PATCH/DELETE /roles/{id}` (`move_to` when the
  role has users; the Administrator role can't be changed, built-in roles can't be deleted:
  `role.builtin_locked`), `GET/PUT /users/{id}/vehicle-access` (assignments with `level` `view` or `edit`;
  they only apply while the user's role uses assigned vehicles) and `GET /vehicles/{id}/access` (who sees a
  vehicle, and how). Changes apply on each affected user's next request.

## Errors

Errors are JSON with a dotted code and never prose, so each client translates them:

```json
{"error": {"code": "validation.failed", "fields": {"odometer_km": "validation.required"}}}
```

Codes every operation can return are listed in the contract's description; `x-torq-error-codes` on each
operation lists its specific ones, and `x-torq-access` says who may call it (`public`, `member`, `admin`).
