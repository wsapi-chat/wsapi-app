# Event Delivery

WSAPI delivers real-time events when things happen on WhatsApp — incoming messages, read receipts, group changes, presence updates, and more. Events are delivered via **webhooks** (HTTP POST) or **Redis Streams**.

## Event Envelope

Every event is wrapped in a standard envelope:

```json
{
  "eventId": "evt_1709472000000_a1b2c3d4e5f6g7h8",
  "instanceId": "my-instance",
  "eventType": "message",
  "receivedAt": "2026-02-17T12:00:00Z",
  "data": { ... }
}
```

| Field | Description |
|-------|-------------|
| `eventId` | Unique ID in format `evt_<unix_ms>_<random_hex>` |
| `instanceId` | Instance that produced the event |
| `eventType` | Event type string (see table below) |
| `receivedAt` | RFC 3339 timestamp |
| `data` | Event-specific payload |

## Event Types

### Message Events

| Type | Description |
|------|-------------|
| `message` | New message received or sent |
| `message_read` | Message read receipt |
| `message_delete` | Message deleted |
| `message_star` | Message starred or unstarred |
| `message_history_sync` | Batch of historical messages (on-demand or flush) |

### Session Events

| Type | Description |
|------|-------------|
| `logged_in` | Device paired successfully |
| `logged_out` | Device logged out (any cause) |
| `login_error` | Pairing error occurred |
| `initial_sync_finished` | Initial history sync completed (only when `historySync` is enabled) |

### Chat Events

| Type | Description |
|------|-------------|
| `chat_setting` | Chat mute, pin, or archive state changed |
| `chat_presence` | Chat typing/recording indicator |
| `chat_push_name` | Contact push name changed |
| `chat_status` | Contact status text changed |
| `chat_picture` | Chat or contact picture changed |

### Group Events

| Type | Description |
|------|-------------|
| `group` | Group info changed (name, description, participants, settings) |

### Contact Events

| Type | Description |
|------|-------------|
| `contact` | Contact synced from WhatsApp |

### Call Events

| Type | Description |
|------|-------------|
| `call_offer` | Incoming call received |
| `call_terminate` | Call ended |
| `call_accept` | Call accepted |

### Presence Events

| Type | Description |
|------|-------------|
| `user_presence` | User online/offline presence update |

### Newsletter Events

| Type | Description |
|------|-------------|
| `newsletter` | Newsletter update received |

## System Events

Four event types are **system events** — they are always delivered regardless of event filter configuration:

- `logged_in`
- `logged_out`
- `login_error`
- `initial_sync_finished`

System events cannot be filtered out. If included in `eventFilters`, they are silently stripped.

### `logged_out` Guaranteed Delivery

The `logged_out` event is published under all logout circumstances:

| Cause | Reason field | Trigger |
|-------|-------------|---------|
| Server-side logout (401, device removed from phone) | `"server_logout"` | whatsmeow `LoggedOut` event |
| API logout (`POST /session/logout`) | `"api_logout"` | `Manager.HandleLogout()` |
| Instance deletion (`DELETE /instances/{id}`) | `"instance_deleted"` | `Manager.DeleteInstance()` |

## Event Filtering

Instances can specify `eventFilters` — a list of event type strings. When filters are set, only matching event types are published (plus system events which always pass through).

An empty filter list means **all events** are published.

**Example: only receive messages and read receipts:**

```bash
curl -X PUT http://localhost:8080/admin/instances/my-instance/config \
  -H "X-Api-Key: my-admin-key" \
  -H "Content-Type: application/json" \
  -d '{"eventFilters": ["message", "message_read"]}'
```

Or via environment variable:

```bash
WSAPI_DEFAULT_EVENT_FILTERS=message,message_read
```

## Webhook Delivery

Set `WSAPI_PUBLISH_VIA=webhook` and configure a webhook URL.

**Configuration:**

```bash
WSAPI_PUBLISH_VIA=webhook
WSAPI_DEFAULT_WEBHOOK_URL=https://example.com/webhook
WSAPI_DEFAULT_SIGNING_SECRET=my-secret   # optional
```

**Delivery behavior:**

1. Event is marshaled to JSON
2. If a signing secret is configured, HMAC-SHA256 is computed over the JSON body
3. HTTP POST to the webhook URL with headers:
   - `Content-Type: application/json`
   - `X-Webhook-Signature: sha256=<hex>` (if signing secret configured)
4. Fire-and-forget — errors are logged but not retried

### Webhook Signature Verification

When `signingSecret` is set, verify the signature to ensure the webhook came from WSAPI:

**Node.js example:**

```javascript
const crypto = require('crypto');

function verifySignature(body, signature, secret) {
  const expected = 'sha256=' + crypto
    .createHmac('sha256', secret)
    .update(body)
    .digest('hex');
  return crypto.timingSafeEqual(
    Buffer.from(signature),
    Buffer.from(expected)
  );
}

// In your webhook handler:
const signature = req.headers['x-webhook-signature'];
const isValid = verifySignature(req.rawBody, signature, 'my-secret');
```

**Python example:**

```python
import hmac
import hashlib

def verify_signature(body: bytes, signature: str, secret: str) -> bool:
    expected = 'sha256=' + hmac.new(
        secret.encode(), body, hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(signature, expected)
```

**Go example:**

```go
func verifySignature(body []byte, signature, secret string) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(signature), []byte(expected))
}
```

## Redis Streams

Set `WSAPI_PUBLISH_VIA=redis` and configure Redis connection details.

**Configuration:**

```bash
WSAPI_PUBLISH_VIA=redis
WSAPI_REDIS_URL=localhost:6379
WSAPI_REDIS_PASSWORD=my-password      # optional
WSAPI_REDIS_STREAM_NAME=my-stream     # optional, default: stream:<instanceId>
```

Events are published via `XADD` with the following fields:

| Field | Description |
|-------|-------------|
| `eventId` | Unique event ID |
| `instanceId` | Instance that produced the event |
| `eventType` | Event type string |
| `receivedAt` | RFC 3339 timestamp |
| `eventData` | JSON string of the `data` payload only |
| `signature` | HMAC-SHA256 signature (only when `signingSecret` is configured) |

**Reading events:**

```bash
# Read new events from the stream
redis-cli XREAD COUNT 10 BLOCK 5000 STREAMS stream:my-instance $

# Read all events from the beginning
redis-cli XRANGE stream:my-instance - +
```

### Redis Sentinel

For high availability, configure Redis Sentinel mode:

```bash
WSAPI_REDIS_MODE=sentinel
WSAPI_REDIS_URL=sentinel1:26379,sentinel2:26379,sentinel3:26379
WSAPI_REDIS_MASTER_NAME=mymaster
WSAPI_REDIS_SENTINEL_PASSWORD=sentinel-pass
```

## Publisher Selection

The publisher is selected based on `WSAPI_PUBLISH_VIA` (or `eventsPublishVia` in config.yaml):

| Value | Publisher | Fallback |
|-------|-----------|----------|
| `"webhook"` | Webhook (HTTP POST) | Noop if no webhook URL configured |
| `"redis"` | Redis Streams (XADD) | Noop if Redis not configured |
| `"none"` / empty | Noop (debug logging only) | — |

## Full Event Payload Schemas

See the [events OpenAPI spec](https://github.com/wsapi-chat/wsapi-app/blob/main/openapi/wsapi-events.yml) for complete payload schemas for all event types. [![Swagger UI](https://img.shields.io/badge/Swagger_UI-85EA2D?logo=swagger&logoColor=black)](https://petstore.swagger.io/?url=https://raw.githubusercontent.com/wsapi-chat/wsapi-app/main/openapi/wsapi-events.yml)
