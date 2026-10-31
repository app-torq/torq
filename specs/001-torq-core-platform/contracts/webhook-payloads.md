# Contract: Outbound Webhook Payloads

**Feature**: [spec.md](../spec.md) (FR-035 to FR-043) | **Research**: R10, R12, R13

All requests use `POST` with `User-Agent: torq/<version>`, plus any custom headers configured on the endpoint. Timeout is 10 s and redirects are not followed.

All text is rendered in the endpoint owner's locale (`en` or `de`, with English fallback), using their date and number format. Units are always metric.

## Events

| Event | Fires when | Severity |
|---|---|---|
| `reminder_due_soon` | A reminder changes state to `due_soon` (see data-model §6) | `warning` |
| `reminder_overdue` | A reminder changes state to `overdue` | `critical` |
| `digest_weekly` / `digest_monthly` | At the configured local day and hour, once per period per endpoint | `info` |
| `test` | The "Send Test Notification" action | `info` |

## Generic JSON (`generic_json`), also the Home Assistant body

```jsonc
{
  "schema": "torq.notification/v1",
  "event": "reminder_overdue",
  "severity": "critical",                 // info | warning | critical
  "sent_at": "2026-10-30T08:00:00Z",
  "title": "Overdue: Engine Oil & Filter — Mercedes-Benz C 220 CDI",      // localized
  "message": "Overdue by 350 km (due at 225.000 km).",                    // localized
  "url": "https://torq.example/vehicles/1/reminders/7",                  // omitted if TORQ_BASE_URL unset
  "reminder": {                            // reminder events only
    "id": 7,
    "name": "Engine Oil & Filter",
    "state": "overdue",
    "trigger": "km",                       // km | time
    "next_due_km": 225000,
    "next_due_date": "2027-10-15",
    "remaining_km": -350,
    "remaining_days": 300
  },
  "vehicle": {"id": 1, "name": "Mercedes-Benz C 220 CDI", "license_plate": "B-TQ 646", "current_odometer_km": 225350},
  "digest": {                              // digest events only
    "period": "weekly",
    "overdue":  [ /* reminder objects, each with a nested vehicle object */ ],
    "due_soon": [ /* … */ ]
  }
}
```

Numeric fields always use plain JSON numbers. Locale formatting is applied only to `title` and `message`.

An empty digest sends `overdue: []` and `due_soon: []`, with the message "Nothing due" (localized).

## Discord (`discord`)

```jsonc
{
  "username": "torq",
  "embeds": [{
    "title": "<title>",
    "description": "<message>",
    "url": "<url if set>",
    "color": 15680580,                   // red for critical, 16096779 amber for warning, 7434609 zinc for info
    "fields": [
      {"name": "<i18n: Vehicle>", "value": "Mercedes-Benz C 220 CDI", "inline": true},
      {"name": "<i18n: Odometer>", "value": "225.350 km", "inline": true}
    ],
    "timestamp": "<sent_at>"
  }]
}
```

Digest messages list up to 20 reminders as fields. If there are more, the description ends with "+N more".

## ntfy (`ntfy`)

- **URL**: the full topic URL.
- **Body**: the `message` as plain text (`Content-Type: text/plain; charset=utf-8`). Digests add one line per reminder.
- **Headers**:
  - `Title`: the title, RFC 2047-encoded (`=?UTF-8?B?…?=`) when it contains non-ASCII characters
  - `Priority`: `3` for info, `4` for warning, `5` for critical
  - `Tags`: `wrench` for info, `warning` for warning, `rotating_light` for critical
  - `Click`: the deep link, only when `TORQ_BASE_URL` is set

Users add `Authorization: Bearer <token>` as a custom header.

## Gotify (`gotify`)

- **URL**: `https://gotify.example/message`.
- **Token**: the user supplies it either as the custom header `X-Gotify-Key` or as `?token=` in the URL.

```jsonc
{
  "title": "<title>",
  "message": "<message, markdown>",
  "priority": 8,                       // 2 for info, 5 for warning, 8 for critical
  "extras": {
    "client::display": {"contentType": "text/markdown"},
    "client::notification": {"click": {"url": "<url>"}}   // only when the base URL is set
  }
}
```

## Home Assistant (`home_assistant`)

- **URL**: the webhook trigger URL, `http://homeassistant.local:8123/api/webhook/<webhook_id>`.
- **Body**: identical to Generic JSON. Automations read it as `{{ trigger.json.title }}`, `{{ trigger.json.reminder.state }}`, and so on.

## Test payload

- **Shape**: the same as the preset's normal payload, with `event: "test"` and `severity: "info"`.
- **Content**: a sample reminder taken from the first vehicle in the garage, or placeholder data if the garage is empty.
- **Title**: "torq test notification" (localized).

## Compatibility

Changes to the `torq.notification/v1` fields are additive only. A breaking change requires `v2` and the matching preset version.
