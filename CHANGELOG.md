# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [3.2.0] - 2026-11-25

### Added

- **Passkey pairing** — some WhatsApp accounts now require the owner's passkey (WebAuthn) to link a companion device. New endpoints: `GET /session/passkey` (current step and the WebAuthn challenge to sign on `https://web.whatsapp.com`), `POST /session/passkey` (submit `credential.toJSON()`; `type`, `origin` and `challenge` are checked locally before forwarding), `POST /session/passkey/request` (ask WhatsApp for a challenge directly, which also allows linking with no QR at all) and `POST /session/passkey/confirm` (compatibility only — the confirmation is sent automatically). `GET /session/status` now reports `passkeyStatus`. While a step is pending, `GET /session/qr`, `GET /session/qr/text` and `GET /session/pair-code/{phone}` return `409` instead of disconnecting the session, and QR rotation is paused
- `verifiedName` on `GET /users/{phone}/profile` for verified WhatsApp Business accounts; `isVerified` is no longer always `false`
- `whatsmeow.sendTimeout` (`WSAPI_WHATSMEOW_SEND_TIMEOUT`, default `20s`) bounds how long a send waits for WhatsApp's acknowledgement
- History sync chunks are logged at info level with their type, order, progress, compressed size and conversation count

### Changed

- Groups can be created with only the paired account by omitting `participants` or passing an empty array to `POST /groups`
- When an instance has history sync off, pairing asks the phone for 1 day / 10 MB of history instead of the full history. Already paired devices are unaffected
- `DELETE /admin/instances/{id}` runs on its own 2-minute deadline (30 seconds for the WhatsApp logout), so it completes even if the caller times out. A delete that arrives while another is running returns `409` instead of `404`; retry until it returns `204`
- Debug logs attach event payloads only up to 16 KiB and log the size above that
- **whatsmeow updated to 2026-11-25**, plus minor dependency updates. whatsmeow migrates `whatsmeow_device` from schema 15 to 16 on first boot; back up the session database before upgrading. Encrypted contact statuses (sent by Android since early November) are no longer dropped and now arrive as message events. The blocklist resolves phone numbers from the server response

### Fixed

- Sends and chat picture lookups waited up to whatsmeow's 75s fallback when WhatsApp never acknowledged; they now fail at `sendTimeout` with a named error
- `POST /session/flush-history` loaded a chat's whole cached backlog into memory; rows are now streamed
- An app state full sync (e.g. right after pairing) replayed every past "delete for me" and star action as live `message_delete` / `message_star` events
- `replyTo.sender.isMe` is now `true` when a contact replies to a message sent by the paired account

## [3.0.0] - 2026-09-28

### Added

- **Click-to-WhatsApp ad attribution** — message events carry an `adReferral` object when a contact writes after clicking a Meta ad (`ctwaClid`, `sourceType`, `sourceId`, `sourceUrl`, `sourceApp`, `title`, `body`, `mediaType`, `thumbnailUrl`, `conversionSource`, `showAdAttribution`). Only emitted when the message carries real attribution data

### Changed

- Context info projection is shared by every message type — media messages now report `ephemeralExpiration`

## [2.1.0] - 2026-09-15

### Added

- **`publishEvents` per-instance flag** — set to `false` for an instance with no delivery target so its data events are not published only to be discarded. System events (`logged_in`, `logged_out`, `login_error`, `initial_sync_finished`) are always published. Unset means enabled, so existing instances are unaffected
- **Bounded media downloads** — `WSAPI_MEDIA_MAX_CONCURRENT_DOWNLOADS` (default 2, 0 disables) caps concurrent `/media/download` decrypts process-wide, with a 2-minute per-download timeout
- **Recipient validation** — `to`, `chatId` and `senderId` are validated before a send, rejecting addresses that cannot route instead of burning the full 75s ack budget

### Changed

- **Go 1.26 is now required**, along with a whatsmeow upgrade to 2026-09-09. whatsmeow migrates `whatsmeow_device` from schema 14 to 15 on first boot; back up the session database before upgrading, rolling back afterwards is not clean
- **`DELETE /admin/instances/{id}` answers `204` instead of `404`** when the instance exists in neither the store nor memory, making the endpoint idempotent
- Default HTTP write timeout raised from 60s to 90s so it outlasts whatsmeow's 75s send-ack budget
- `PublisherFactory.Create` takes an additional `publishEvents` argument (breaking for out-of-tree implementations)
- Phone validation rejects national numbers given an international prefix, since no country code begins with zero
- CI runs tests with `-race`

### Fixed

- **Stalled sends returned `400`** instead of `504`, telling callers their request was malformed and discouraging retries on a transient upstream failure
- **QR pairing could wedge an instance permanently** — reading one code and abandoning the channel let a blocking send park while holding whatsmeow's event-handler read lock, after which the client stopped handling nodes of any tag until restart
- **Instance deletes could strand a row** — the in-memory entry was dropped before the store row, so a failed store delete left the row unreachable. The manager lock also covered network calls, blocking every other instance operation for the duration
- Status updates (`PUT /users/me/profile`) stopped changing the About text after the whatsmeow upgrade
- Groups created through `POST /groups` reported `isEphemeral: true` with no disappearing timer set

### Security

- `golang.org/x/image` v0.45.0, fixing four vulnerabilities reachable from decoding client-supplied images (GO-2026-5031, GO-2026-5032, GO-2026-5062, GO-2026-5066)

## [2.0.1] - 2026-03-05

### Added

- **Instance modes** — `WSAPI_INSTANCE_MODE` config (`"single"` or `"multi"`) controls how instances are managed
- **Single mode** (default) — a fixed `"default"` instance is set up at startup with no app store needed; device session resolved from whatsmeow DB via `GetFirstDevice()`; `X-Instance-Id` header not required
- **Multi mode** — instances managed via `/admin/instances` endpoints; app store (`WSAPI_DB_*`) required for persistence; `X-Instance-Id` header required on all instance endpoints
- `SingleInstanceAuth` middleware for single mode — resolves the fixed instance without requiring `X-Instance-Id`
- `EnsureSingleInstance()` in instance manager for single mode startup provisioning
- Wiki documentation pages covering getting started, instance modes, configuration, authentication, API overview, event delivery, Docker deployment, database setup, and history sync

### Changed

- App store (`wsapi.db`) is only opened in multi mode; skipped entirely in single mode
- Instance manager accepts `nil` store in single mode with nil-guarded device state tracking
- Route registration is conditional — admin endpoints only registered in multi mode
- Updated OpenAPI spec, config examples, and environment variable documentation to reflect instance mode support

## [2.0.0] - 2025-03-04

### Added

- Multi-instance WhatsApp connections with isolated sessions, credentials, and event delivery
- Full REST API: messages (text, media, documents, stickers, contacts, locations, links, reactions), edit, delete, read receipts, pin, star
- Group management: create, participants, settings, invite links, join/leave
- Community management: create, sub-groups, participants
- Contact management: list, get, create/update, sync, block/unblock, blocklist
- Chat operations: list, info, presence, ephemeral, mute, pin, archive, read, clear, delete, business profiles
- User account management: profile (name, status, picture), presence, privacy settings
- User lookup: WhatsApp registration check (single and bulk), profiles with picture URLs
- Newsletter support: list subscribed, get info, create, subscribe/unsubscribe, mute
- Status (stories): post text/image/video updates, delete/revoke
- Media download by encoded media ID
- Call rejection for incoming calls
- Session management: connect, disconnect, QR code, pair code, logout
- History sync: cache and flush history messages on demand
- Event delivery via webhook (HTTP POST with optional HMAC-SHA256 signing) or Redis Streams (XADD)
- Event filtering per instance with system events always delivered
- Dual storage support: SQLite and PostgreSQL for both app store and device sessions
- Local chat, contact, and history sync stores with automatic lifecycle cleanup
- API key authentication for admin and instance endpoints
- HTTP proxy support for outbound requests
- PII redaction in structured logs
- Docker support with multi-stage build
- OpenAPI specifications for API endpoints and event schemas
