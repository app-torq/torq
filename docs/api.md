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

## Errors

Errors are JSON with a dotted code and never prose, so each client translates them:

```json
{"error": {"code": "validation.failed", "fields": {"odometer_km": "validation.required"}}}
```

Codes every operation can return are listed in the contract's description; `x-torq-error-codes` on each
operation lists its specific ones, and `x-torq-access` says who may call it (`public`, `member`, `admin`).
