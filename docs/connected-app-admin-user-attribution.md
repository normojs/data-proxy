# Connected App admin credentials + user attribution

## Product model (developer apply + admin approve)

1. **Admin can create apps directly** (`POST /api/connected-apps`) or approve developer requests (`POST /api/connected-apps/requests/:id/review`).
2. Each app has a public **`client_id`** (defaults to slug) and optional **`client_secret`** (`capp_…`) for confidential clients.
3. **Public clients** use authorization code + PKCE only. **Confidential clients** may send `client_secret` at the token endpoint.
4. Users authorize many apps via **`connected_app_grants`** (not a single user column).
5. Optional first-touch channel: **`users.signup_connected_app_id`** (set once on first OAuth/device/grant path).

## Admin create / edit

Request fields:

- `slug`, `name`, `description`
- `allowed_scopes`, `default_scopes`
- `authorization_flow`: `device_code` | `authorization_code` | `both`
- `redirect_uris`: string array (required for website OAuth flows)
- `client_type`: `public` | `confidential`
- `rotate_secret`: bool (edit only)
- `trusted`, `status`

Response includes `client_id`, `redirect_uris`, `has_client_secret`, and when generated:

- `client_secret` (plaintext once)
- `client_secret_once: true`

## Approve developer request

Review payload may include `client_type: "confidential"` to issue a one-time secret on approve. Website flows still require `callback_url` / redirect.

## User list filters

`GET /api/user/search`:

- `connected_app_id` — users with an **authorized** grant for that app
- `signup_app_id` — `users.signup_connected_app_id = ?`

Admin Users UI toolbar exposes both filters.

## Profile: authorized apps

- `GET /api/user/connected-app-grants` — list current user's grants (+ app name/slug)
- `DELETE /api/user/connected-app-grants/:app_id` — revoke grant, disable bound tokens, revoke access tokens

Profile page shows **Authorized applications** card (in addition to Snapless device card).

## Security notes

- `client_id` is public; safe in frontend configs.
- `client_secret` is hashed at rest (`client_secret_hash`); plaintext only on create/rotate/approve.
- Never ship `client_secret` in mobile/web SPA; use public + PKCE for those.
- Redirect URIs must be absolute `http`/`https` and exact-match.
