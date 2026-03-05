# History Sync

When a new device pairs with WhatsApp, the phone sends historical messages to the newly linked device. WSAPI can capture these messages and make them available via the API.

## Overview

By default, history sync messages are **not** published as events — they are only used internally for chat tracking. When `historySync` is enabled for an instance, WSAPI caches the synced messages and lets you retrieve them on demand.

## Enabling History Sync

### Via environment variable (applies to all instances)

```bash
WSAPI_DEFAULT_HISTORY_SYNC=true
```

### Via admin API (per-instance, multi mode)

```bash
curl -X POST http://localhost:8080/admin/instances \
  -H "X-Api-Key: my-admin-key" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "my-instance",
    "historySync": true
  }'
```

Or update an existing instance:

```bash
curl -X PUT http://localhost:8080/admin/instances/my-instance/config \
  -H "X-Api-Key: my-admin-key" \
  -H "Content-Type: application/json" \
  -d '{"historySync": true}'
```

## How It Works

```
1. Instance created with historySync: true
2. Device pairs -> WhatsApp sends history sync data
3. WSAPI intercepts sync events:
   a. Upserts chats into ChatStore (always happens, even without historySync)
   b. Projects messages and caches them in wsapi_history_sync_messages table
4. RECENT sync completes 
   -> WSAPI publishes initial_sync_finished event
5. Consumer receives initial_sync_finished
   -> Calls POST /session/flush-history
6. WSAPI reads cached messages, groups by chat, publishes as
   message_history_sync events (chunked at 500 messages per event)
7. Cache is cleared after flush
```

## Events

### `initial_sync_finished`

Published **once** after the initial history sync completes (RECENT sync progress >= 100%). Only emitted when `historySync` is enabled. This event signals that the cache is fully populated and ready to be flushed.

```json
{
  "eventId": "evt_1709472000000_a1b2c3d4e5f6g7h8",
  "instanceId": "my-instance",
  "eventType": "initial_sync_finished",
  "receivedAt": "2026-02-18T12:00:00Z",
  "data": {}
}
```

### `message_history_sync`

Published when history sync messages are flushed (via API) or received on-demand. Messages are grouped by chat, with a maximum of 500 messages per event.

```json
{
  "eventId": "evt_1709472000000_a1b2c3d4e5f6g7h8",
  "instanceId": "my-instance",
  "eventType": "message_history_sync",
  "receivedAt": "2026-02-18T12:00:00Z",
  "data": {
    "chatId": "1234567890@s.whatsapp.net",
    "messages": [
      {
        "id": "ABCDEF1234567890",
        "chatId": "1234567890@s.whatsapp.net",
        "sender": {
          "id": "1234567890@s.whatsapp.net",
          "isMe": false
        },
        "isGroup": false,
        "time": "2026-02-18T11:30:00Z",
        "type": "text",
        "text": "Hello!"
      }
    ]
  }
}
```

## API Usage

### Flush cached history

After receiving the `initial_sync_finished` event, call this endpoint to publish the cached messages as `message_history_sync` events:

```bash
curl -X POST http://localhost:8080/session/flush-history \
  -H "X-Instance-Id: my-instance" \
  -H "X-Api-Key: my-secret-key"
```

The endpoint returns `200 OK` immediately. Messages are published asynchronously — your webhook or Redis stream will receive `message_history_sync` events grouped by chat.

### Request on-demand history for a specific chat

You can also request history for a specific chat at any time (the device must be paired):

```bash
curl -X POST http://localhost:8080/chats/1234567890@s.whatsapp.net/messages \
  -H "X-Instance-Id: my-instance" \
  -H "X-Api-Key: my-secret-key"
```

This triggers an on-demand history sync from WhatsApp, which produces `message_history_sync` events directly (without caching).

## Cache Details

- Cached messages are stored in the `wsapi_history_sync_messages` table in the whatsmeow database
- Cache entries have a **1-hour TTL** — expired rows are lazily cleaned up
- The cache is cleared after a successful flush
- Messages are deduplicated by ID during flush
- The `initial_sync_finished` event fires once per pairing (resets on each new `PairSuccess`)

## Typical Consumer Flow

```
1. Create instance with historySync: true
2. Pair device (QR or pair code)
3. Wait for logged_in event
4. Wait for initial_sync_finished event
5. POST /session/flush-history
6. Process message_history_sync events from webhook/Redis
7. Instance is ready for real-time events
```
