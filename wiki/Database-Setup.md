# Database Setup

WSAPI uses a **single database** for everything — WhatsApp device sessions (managed by whatsmeow), instance records, chats, contacts, and history sync cache. Both SQLite and PostgreSQL are supported.

## Database Contents

The database contains whatsmeow's own tables (device sessions, encryption keys, etc.) plus WSAPI custom tables:

| Table | Purpose |
|-------|---------|
| `wsapi_chats` | Tracked chats with last activity timestamps |
| `wsapi_contacts` | Synced contacts with name and addressbook info |
| `wsapi_history_sync_messages` | Cached history sync messages (1-hour TTL) |
| `wsapi_instances` | Instance records (multi mode only) |

The `wsapi_chats`, `wsapi_contacts`, and `wsapi_history_sync_messages` tables have a foreign key on `our_jid` referencing `whatsmeow_device(jid)` with `ON DELETE CASCADE`, so records are automatically cleaned up when a device is deleted or logged out.

## SQLite (Default)

SQLite is the default — zero configuration, no external dependencies.

```bash
# Defaults (no config needed)
WSAPI_DB_DRIVER=sqlite
WSAPI_DB_DSN=./data/wsapi.db
```

The database is stored as a single file in the `./data/` directory. When running with Docker, mount this directory as a volume to persist data.

## PostgreSQL

PostgreSQL is recommended for production deployments, especially with multiple instances.

```bash
WSAPI_DB_DRIVER=postgres
WSAPI_DB_DSN="postgres://user:pass@localhost:5432/wsapi?sslmode=disable"
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
      POSTGRES_DB: wsapi
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U wsapi"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  pgdata:
```

## Migrations

WSAPI handles database migrations automatically on startup — no manual steps needed.

- **WSAPI custom table migrations** are tracked in the `wsapi_version` table
- whatsmeow's own migrations are managed internally by the library
