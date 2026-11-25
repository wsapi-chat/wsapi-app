# Upgrading

## OSS 2.1.0

Before starting the new binary or image, back up the complete session database and instance configuration. Stop the service while taking a consistent SQLite backup, or use a consistent PostgreSQL database backup.

- Building from source requires Go 1.26.
- On first startup, whatsmeow migrates its device schema from 14 to 15. Replacing the binary with an older version does not reverse this migration. Rollback requires the corresponding pre-upgrade database backup; do not run old and new versions against the same database.
- The default HTTP write timeout is 90 seconds. Upstream send timeouts return HTTP 504; malformed recipients are rejected before sending. Avoid blindly retrying a message after a timeout because delivery may be uncertain.
- Deleting an already absent instance through `DELETE /admin/instances/{id}` returns 204.
- `publishEvents: false` suppresses data events, but not system events. Unset remains enabled.
- Media decrypt concurrency defaults to 2 across the process; `WSAPI_MEDIA_MAX_CONCURRENT_DOWNLOADS=0` disables the cap.

## OSS 3.0.0

- Optional Click-to-WhatsApp `adReferral` metadata is projected for text and media messages. Media messages also report `ephemeralExpiration`.

## OSS 3.2.0

Before starting the new binary or image, back up the session database: on first startup whatsmeow migrates its device schema from 15 to 16, and an older binary cannot use the migrated database.

- Passkey pairing: new `/session/passkey` endpoints and `passkeyStatus` on `GET /session/status`. Clients that poll `GET /session/qr` or `GET /session/pair-code/{phone}` must handle `409` and stop polling while `passkeyStatus` is `required`, `submitted` or `confirmation_required`.
- `DELETE /admin/instances/{id}` can return `409` while another delete of the same instance is running; retry until `204`.
- `POST /groups` no longer requires `participants`.
- Sends fail after `whatsmeow.sendTimeout` (default 20s) instead of whatsmeow's 75s fallback.
- Instances with history sync off request only 1 day / 10 MB of history when pairing. Enable `historySync` before pairing if you need the full history.

Cloud Account APIs and SSE delivery are separate Cloud features. The OSS administrative API remains `/admin/instances`; event delivery is via webhooks or Redis Streams.

## Unreleased

- Media `url` fields that resolve to loopback, private, link-local, CGNAT, unspecified or multicast addresses are rejected with `400`, and only `http`/`https` URLs are accepted. If you send media from a server on your own network, set `WSAPI_MEDIA_ALLOW_PRIVATE_URLS=true` (`mediaAllowPrivateUrls: true`) and make sure every API caller is trusted.
