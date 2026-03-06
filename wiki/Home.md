# WSAPI

A Go REST API for WhatsApp built on [whatsmeow](https://github.com/tulir/whatsmeow). Run multiple WhatsApp sessions behind a single HTTP API with webhook or Redis event delivery.

## Features

- **Multi-instance** — run multiple WhatsApp sessions in one process, each with its own credentials, webhook, and event filters
- **Full REST API** — 74 endpoints covering messages, groups, communities, contacts, chats, newsletters, status updates, calls, and more
- **Event delivery** — receive real-time events via webhook (HTTP POST with optional HMAC-SHA256 signing) or Redis Streams
- **Flexible storage** — SQLite (zero-config) or PostgreSQL, single database for everything
- **Docker ready** — single-container deployment, available on Docker Hub
- **OpenAPI documented** — complete API and event specs in `openapi/`

## Documentation

| Page | Description |
|------|-------------|
| [Getting Started](Getting-Started) | Installation, quick-start walkthrough |
| [Instance Modes](Instance-Modes) | Single vs multi mode |
| [Configuration](Configuration) | Config file, environment variables |
| [Authentication](Authentication) | API key auth for admin and instance endpoints |
| [API Overview](API-Overview) | All 74 endpoints grouped by domain |
| [Event Delivery](Event-Delivery) | Webhooks, Redis Streams, event types, filtering |
| [Docker Deployment](Docker-Deployment) | Docker run, Compose, volumes, production tips |
| [Database Setup](Database-Setup) | SQLite vs PostgreSQL, database schema |
| [History Sync](History-Sync) | Pairing history sync, on-demand retrieval |

## Quick Links

- [OpenAPI spec — REST API](https://github.com/wsapi-chat/wsapi-app/blob/main/openapi/wsapi-api.yml) [![Swagger UI](https://img.shields.io/badge/Swagger_UI-85EA2D?logo=swagger&logoColor=black)](https://petstore.swagger.io/?url=https://raw.githubusercontent.com/wsapi-chat/wsapi-app/main/openapi/wsapi-api.yml)
- [OpenAPI spec — Events](https://github.com/wsapi-chat/wsapi-app/blob/main/openapi/wsapi-events.yml) [![Swagger UI](https://img.shields.io/badge/Swagger_UI-85EA2D?logo=swagger&logoColor=black)](https://petstore.swagger.io/?url=https://raw.githubusercontent.com/wsapi-chat/wsapi-app/main/openapi/wsapi-events.yml)
- [Example config](https://github.com/wsapi-chat/wsapi-app/blob/main/config.example.yaml)
- [Contributing guide](https://github.com/wsapi-chat/wsapi-app/blob/main/CONTRIBUTING.md)
- [License (MIT)](https://github.com/wsapi-chat/wsapi-app/blob/main/LICENSE)
