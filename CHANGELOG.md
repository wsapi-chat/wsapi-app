# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

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
