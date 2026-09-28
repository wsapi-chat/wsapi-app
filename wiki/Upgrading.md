# Upgrading

## OSS 2.1.0

Before starting the new binary or image, back up the complete session database and instance configuration. Stop the service while taking a consistent SQLite backup, or use a consistent PostgreSQL database backup.

- Building from source requires Go 1.26.
- On first startup, whatsmeow migrates its device schema from 14 to 15. Replacing the binary with an older version does not reverse this migration. Rollback requires the corresponding pre-upgrade database backup; do not run old and new versions against the same database.
- The default HTTP write timeout is 90 seconds. Upstream send timeouts return HTTP 504; malformed recipients are rejected before sending. Avoid blindly retrying a message after a timeout because delivery may be uncertain.
- Deleting an already absent instance through `DELETE /admin/instances/{id}` returns 204.
- `publishEvents: false` suppresses data events, but not system events. Unset remains enabled.
- Media decrypt concurrency defaults to 2 across the process; `WSAPI_MEDIA_MAX_CONCURRENT_DOWNLOADS=0` disables the cap.

## Unreleased source changes

Optional Click-to-WhatsApp `adReferral` metadata is now projected for text and media messages and represented in event schemas. Shared context projection also preserves media ephemeral expiration. These changes require a future OSS release; they are not included in the published 2.1.0 image.

Cloud Account APIs and SSE delivery are separate Cloud features. The OSS administrative API remains `/admin/instances`; event delivery is via webhooks or Redis Streams.
