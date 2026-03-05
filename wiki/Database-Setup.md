# Database Setup

WSAPI uses **two independent databases** — one for the app store (instance records) and one for WhatsApp device sessions. Both support SQLite and PostgreSQL.

## Two-Database Architecture

| Database | Purpose | Config Variables | Used In |
|----------|---------|------------------|---------|
| **App Store** | Instance records, config persistence | `WSAPI_DB_DRIVER`, `WSAPI_DB_DSN` | Multi mode only |
| **WhatsApp Store** | Device sessions, keys, contacts, chats, history sync cache | `WSAPI_WHATSMEOW_DB_DRIVER`, `WSAPI_WHATSMEOW_DB_DSN` | Both modes |

### App Store

The app store persists instance records — ID, API key, webhook URL, signing secret, event filters, history sync setting, device state, and timestamps. It is managed by WSAPI's `internal/store` package.

**Multi mode only.** In single mode, the app store is not opened and `wsapi.db` is not created.

### WhatsApp Store

The WhatsApp store is managed by the [whatsmeow](https://github.com/tulir/whatsmeow) library and holds device sessions, encryption keys, and contacts. WSAPI adds custom tables to this database:

| Table | Purpose |
|-------|---------|
| `wsapi_chats` | Tracked chats with last activity timestamps |
| `wsapi_contacts` | Synced contacts with name and addressbook info |
| `wsapi_history_sync_messages` | Cached history sync messages (1-hour TTL) |

All custom tables have a foreign key on `our_jid` referencing `whatsmeow_device(jid)` with `ON DELETE CASCADE`, so records are automatically cleaned up when a device is deleted or logged out.

**Used in both single and multi modes.**

## SQLite (Default)

SQLite is the default — zero configuration, no external dependencies.

```bash
# Defaults (no config needed)
WSAPI_DB_DRIVER=sqlite
WSAPI_DB_DSN=./data/wsapi.db

WSAPI_WHATSMEOW_DB_DRIVER=sqlite
WSAPI_WHATSMEOW_DB_DSN=./data/whatsmeow.db
```

Both databases are stored as files in the `./data/` directory. When running with Docker, mount this directory as a volume to persist data.

### Single mode SQLite

In single mode, only the whatsmeow database is used:

```bash
# Only this is needed
WSAPI_WHATSMEOW_DB_DSN=./data/whatsmeow.db
```

## PostgreSQL

PostgreSQL is recommended for production deployments, especially with multiple instances.

```bash
# App store (multi mode only)
WSAPI_DB_DRIVER=postgres
WSAPI_DB_DSN="postgres://user:pass@localhost:5432/wsapi?sslmode=disable"

# WhatsApp store
WSAPI_WHATSMEOW_DB_DRIVER=postgres
WSAPI_WHATSMEOW_DB_DSN="postgres://user:pass@localhost:5432/whatsmeow?sslmode=disable"
```

### Connection string format

```
postgres://user:password@host:port/database?sslmode=disable
```

| Parameter | Description |
|-----------|-------------|
| `user` | PostgreSQL username |
| `password` | PostgreSQL password |
| `host` | PostgreSQL server hostname |
| `port` | PostgreSQL port (default: 5432) |
| `database` | Database name |
| `sslmode` | SSL mode (`disable`, `require`, `verify-ca`, `verify-full`) |

### Using separate databases

It's recommended to use separate PostgreSQL databases for the app store and whatsmeow store:

```bash
WSAPI_DB_DSN="postgres://wsapi:pass@db:5432/wsapi?sslmode=disable"
WSAPI_WHATSMEOW_DB_DSN="postgres://wsapi:pass@db:5432/whatsmeow?sslmode=disable"
```

### Docker Compose with PostgreSQL

```yaml
services:
  wsapi:
    image: wsapichat/wsapi:latest
    ports:
      - "8080:8080"
    environment:
      WSAPI_INSTANCE_MODE: multi
      WSAPI_DB_DRIVER: postgres
      WSAPI_DB_DSN: "postgres://wsapi:password@db:5432/wsapi?sslmode=disable"
      WSAPI_WHATSMEOW_DB_DRIVER: postgres
      WSAPI_WHATSMEOW_DB_DSN: "postgres://wsapi:password@db:5432/whatsmeow?sslmode=disable"
      WSAPI_ADMIN_API_KEY: my-admin-key
      WSAPI_DEFAULT_API_KEY: my-instance-key
    depends_on:
      db:
        condition: service_healthy
    restart: unless-stopped

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: wsapi
      POSTGRES_PASSWORD: password
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./init-db.sh:/docker-entrypoint-initdb.d/init-db.sh
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U wsapi"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  pgdata:
```

Create `init-db.sh` to set up both databases:

```bash
#!/bin/bash
set -e
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
  CREATE DATABASE wsapi;
  CREATE DATABASE whatsmeow;
EOSQL
```

## Migrations

WSAPI handles database migrations automatically on startup — no manual steps needed.

- **App store migrations** are tracked in the `wsapi_version` table
- **WhatsApp store custom table migrations** are tracked in the `wsapi_wa_version` table
- whatsmeow's own migrations are managed internally by the library

## Which Mode Uses Which Database?

| Database | Single Mode | Multi Mode |
|----------|-------------|------------|
| App Store (`wsapi.db`) | Not used | Required |
| WhatsApp Store (`whatsmeow.db`) | Required | Required |
