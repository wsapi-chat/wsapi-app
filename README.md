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

```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -e WSAPI_ADMIN_API_KEY=my-secret-admin-key \
  -e WSAPI_DEFAULT_API_KEY=my-secret-instance-key \
  wsapichat/wsapi:latest
```

Verify it's running:

```bash
curl http://localhost:8080/health
```

See the [Getting Started](https://github.com/wsapi-chat/wsapi-app/wiki/Getting-Started) guide for the full walkthrough — create an instance, pair your device, and send your first message.

## Documentation

| Page                                                                                | Description                                     |
| ----------------------------------------------------------------------------------- | ----------------------------------------------- |
| [Getting Started](https://github.com/wsapi-chat/wsapi-app/wiki/Getting-Started)     | Installation, quick-start walkthrough           |
| [Instance Modes](https://github.com/wsapi-chat/wsapi-app/wiki/Instance-Modes)       | Single vs multi mode                            |
| [Configuration](https://github.com/wsapi-chat/wsapi-app/wiki/Configuration)         | Config file, environment variables              |
| [Authentication](https://github.com/wsapi-chat/wsapi-app/wiki/Authentication)       | API key auth for admin and instance endpoints   |
| [API Overview](https://github.com/wsapi-chat/wsapi-app/wiki/API-Overview)           | All 74 endpoints grouped by domain              |
| [Event Delivery](https://github.com/wsapi-chat/wsapi-app/wiki/Event-Delivery)       | Webhooks, Redis Streams, event types, filtering |
| [Docker Deployment](https://github.com/wsapi-chat/wsapi-app/wiki/Docker-Deployment) | Docker run, Compose, volumes, production tips   |
| [Database Setup](https://github.com/wsapi-chat/wsapi-app/wiki/Database-Setup)       | SQLite vs PostgreSQL, two-database architecture |
| [History Sync](https://github.com/wsapi-chat/wsapi-app/wiki/History-Sync)           | Pairing history sync, on-demand retrieval       |

OpenAPI specs: [`wsapi-api.yml`](openapi/wsapi-api.yml) (REST API) · [`wsapi-events.yml`](openapi/wsapi-events.yml) (events)

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

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full development guide.

## License

[MIT](LICENSE)
