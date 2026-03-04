# WSAPI

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI](https://github.com/wsapi-chat/wsapi-app/actions/workflows/ci.yml/badge.svg)](https://github.com/wsapi-chat/wsapi-app/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/wsapi-chat/wsapi-app)](https://goreportcard.com/report/github.com/wsapi-chat/wsapi-app)

A Go REST API for WhatsApp built on [whatsmeow](https://github.com/tulir/whatsmeow). Run multiple WhatsApp sessions behind a single HTTP API with webhook or Redis event delivery.

## Features

- **Multi-instance** — run multiple WhatsApp sessions in one process, each with its own credentials, webhook, and event filters
- **Full REST API** — send and receive messages, manage groups, communities, contacts, chats, newsletters, status updates, and more
- **Event delivery** — receive real-time events via webhook (HTTP POST with optional HMAC-SHA256 signing) or Redis Streams
- **Dual storage** — SQLite (zero-config) or PostgreSQL for both app state and device sessions
- **Docker ready** — single-container deployment with Docker Compose
- **OpenAPI documented** — complete API and event specs in `openapi/`

## Quick Start

### 1. Run the server

**Option A: Docker** (from docker hub)

```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -e WSAPI_ADMIN_API_KEY=my-secret-admin-key \
  -e WSAPI_DEFAULT_API_KEY=my-secret-instance-key \
  wsapichat/wsapi:latest
```

**Option B: Docker Compose** (build from source)

```bash
cp config.example.yaml config.yaml   # edit as needed (see Configuration below)
docker compose up --build
```

**Option C: From source** (requires Go 1.25+ and CGO):

```bash
cp config.example.yaml config.yaml   # edit as needed (see Configuration below)
go build -o bin/server ./cmd/server
WSAPI_ADMIN_API_KEY=my-secret-admin-key \
WSAPI_DEFAULT_API_KEY=my-secret-instance-key \
  ./bin/server
```

The server starts on port `8080` with SQLite storage in `./data/`. Verify it's running:

```bash
curl http://localhost:8080/health
```

### 2. Create a WhatsApp instance

Each WhatsApp session is an **instance**. Use the admin API to create one:

```bash
curl -X POST http://localhost:8080/admin/instances \
  -H "X-Api-Key: my-secret-admin-key" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "my-instance"
  }'
```

### 3. Pair your WhatsApp device

Once the instance is created, link it to your WhatsApp account. All instance endpoints require the `X-Instance-Id` and `X-Api-Key` headers (see configuration below to set the ApiKey). 

**Option A: QR code** (scan with your phone):

```bash
# Save the QR code as a PNG image and scan it with WhatsApp > Linked Devices > Link a Device
curl http://localhost:8080/session/qr \
  -H "X-Instance-Id: my-instance" \
  -H "X-Api-Key: my-secret-instance-key" \
  --output qr.png
```

**Option B: Pair code** (enter the code on your phone):

```bash
# Replace the phone number with your own (digits only, no + prefix)
curl http://localhost:8080/session/pair-code/1234567890 \
  -H "X-Instance-Id: my-instance" \
  -H "X-Api-Key: my-secret-instance-key"
# Returns: {"code": "ABCD-EFGH"}
# Enter this code in WhatsApp > Linked Devices > Link with phone number
```

### 4. Verify the connection

```bash
curl http://localhost:8080/session/status \
  -H "X-Instance-Id: my-instance" \
  -H "X-Api-Key: my-secret-instance-key"
# Returns: {"isConnected": true, "isLoggedIn": true, "deviceId": "..."}
```

### 5. Send a message

```bash
curl -X POST http://localhost:8080/messages/text \
  -H "X-Instance-Id: my-instance" \
  -H "X-Api-Key: my-secret-instance-key" \
  -H "Content-Type: application/json" \
  -d '{"to": "1234567890", "text": "Hello from WSAPI!"}'
```

## Configuration

WSAPI is configured via `config.yaml` and/or environment variables. Env vars override file values.

See [`config.example.yaml`](config.example.yaml) and [`.env.example`](.env.example) for all options.

Environment variables:

| Variable                                   | Description                                                                               | Default               |
| ------------------------------------------ | ----------------------------------------------------------------------------------------- | --------------------- |
| `WSAPI_PORT`                               | HTTP port                                                                                 | `8080`                |
| `WSAPI_DB_DRIVER`                          | App store driver (`sqlite`/`postgres`)                                                    | `sqlite`              |
| `WSAPI_DB_DSN`                             | App store DSN                                                                             | `./data/wsapi.db`     |
| `WSAPI_WHATSMEOW_DB_DRIVER`                | whatsmeow DB driver (`sqlite`/`postgres`)                                                 | `sqlite`              |
| `WSAPI_WHATSMEOW_DB_DSN`                   | whatsmeow DB DSN                                                                          | `./data/whatsmeow.db` |
| `WSAPI_ADMIN_API_KEY`                      | API key for `/admin/instances` (sent via `X-Api-Key` header)                              |                       |
| `WSAPI_DEFAULT_API_KEY`                    | Default API key for instance APIs (under `instanceDefaults`, sent via `X-Api-Key` header) |                       |
| `WSAPI_LOG_LEVEL`                          | `debug`, `info`, `warn`, `error`                                                          | `info`                |
| `WSAPI_LOG_FORMAT`                         | `text` or `json`                                                                          | `text`                |
| `WSAPI_LOG_REDACT`                         | Redact sensitive fields in event data logs (`true`/`false`)                               | `true`                |
| `WSAPI_WHATSMEOW_LOG_LEVEL`                | Log level for whatsmeow (under `whatsmeow` config block)                                  | `warn`                |
| `WSAPI_WHATSMEOW_PAIR_CLIENT_TYPE`         | Pair client type (`chrome`, `edge`, `firefox`, `opera`, `safari`)                         | `chrome`              |
| `WSAPI_WHATSMEOW_PAIR_CLIENT_DISPLAY_NAME` | Pair client display name shown during pairing                                             | `Chrome (Windows)`    |
| `WSAPI_HTTP_PROXY`                         | HTTP proxy URL for outbound requests (webhooks, media downloads)                          |                       |
| `WSAPI_DEFAULT_WEBHOOK_URL`                | Default webhook URL for new instances                                                     |                       |
| `WSAPI_DEFAULT_SIGNING_SECRET`             | Default HMAC-SHA256 signing secret                                                        |                       |
| `WSAPI_DEFAULT_EVENT_FILTERS`              | Comma-separated event filter list                                                         |                       |
| `WSAPI_DEFAULT_HISTORY_SYNC`               | Default history sync for new instances (`true`/`false`)                                   |                       |
| `WSAPI_PUBLISH_VIA`                        | Event publish method: `"webhook"`, `"redis"`, or `"none"`                                 |                       |
| `WSAPI_REDIS_MODE`                         | Redis mode: `"standalone"` or `"sentinel"`                                                | `standalone`          |
| `WSAPI_REDIS_URL`                          | Redis address (standalone: `host:port`; sentinel: comma-separated addrs)                  |                       |
| `WSAPI_REDIS_PASSWORD`                     | Redis password                                                                            |                       |
| `WSAPI_REDIS_DB`                           | Redis DB number                                                                           | `0`                   |
| `WSAPI_REDIS_STREAM_NAME`                  | Fixed Redis stream name                                                                   | `stream:<instanceId>` |
| `WSAPI_REDIS_TLS`                          | Enable TLS for Redis connection (`true`/`false`)                                          | `false`               |
| `WSAPI_REDIS_TLS_INSECURE`                 | Skip TLS certificate verification (`true`/`false`)                                        | `false`               |
| `WSAPI_REDIS_MASTER_NAME`                  | Sentinel master name                                                                      | `mymaster`            |
| `WSAPI_REDIS_SENTINEL_PASSWORD`            | Sentinel auth password                                                                    |                       |

## API Documentation

Full OpenAPI specs are available in the [`openapi/`](openapi/) directory:

- [`wsapi-api.yml`](openapi/wsapi-api.yml) — REST API endpoints
- [`wsapi-events.yml`](openapi/wsapi-events.yml) — webhook/event payload schemas

## Authentication

WSAPI uses API key authentication via the `X-Api-Key` header:

- **Admin endpoints** (`/admin/instances/*`) — authenticated with `WSAPI_ADMIN_API_KEY`
- **Instance endpoints** (`/*`) — require `X-Instance-Id` header and the instance's API key (or the default key set via `WSAPI_DEFAULT_API_KEY`)

## Event Delivery

Events are delivered in real time when things happen on WhatsApp (incoming messages, read receipts, group changes, etc.).

**Webhook** — WSAPI sends an HTTP POST to the instance's configured webhook URL. When a `signingSecret` is set, each request includes an `X-Webhook-Signature` header with the HMAC-SHA256 signature of the body.

**Redis Streams** — events are published via `XADD` to a configurable Redis stream (default: `stream:<instanceId>`).

See the [events spec](openapi/wsapi-events.yml) for all event types and payload schemas.


## Development

```bash
make build          # compile binary
make test           # run tests
make lint           # run golangci-lint
make vet            # run go vet
make fmt-check      # check formatting
make openapi-lint   # validate OpenAPI specs
make help           # show all targets
```

CGO is required for the SQLite driver. On macOS this works out of the box. On Linux: `apt install gcc`.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full development guide.

## License

[MIT](LICENSE)
