# Configuration

WSAPI is configured via `config.yaml` and/or environment variables. Environment variables always override config file values.

## Config Hierarchy

```
Defaults (compiled into binary)
  ↓ overridden by
config.yaml (if present)
  ↓ overridden by
Environment variables (WSAPI_*)
```

## Config File

Copy the example config to get started:

```bash
cp config.example.yaml config.yaml
```

### Full Reference

```yaml
server:
  port: 8080
  readTimeout: "30s"
  writeTimeout: "60s"
  shutdownTimeout: "10s"

database:
  driver: "sqlite"           # "sqlite" or "postgres"
  dsn: "./data/wsapi.db"     # SQLite path or PostgreSQL connection string

whatsmeow:
  logLevel: "warn"           # debug, info, warn, error
  pairClientType: "chrome"   # chrome, edge, firefox, opera, safari
  pairClientOs: "Windows"    # OS name shown in WhatsApp Linked Devices

auth:
  adminApiKey: ""            # API key for /admin/instances

logging:
  level: "info"              # debug, info, warn, error
  format: "text"             # text or json
  redactPii: true            # redact sensitive fields from event logs

httpProxy: ""                # HTTP proxy URL for outbound requests

instanceMode: "single"       # "single" or "multi"

instanceDefaults:
  apiKey: ""                 # default API key for instances
  webhookUrl: ""             # default webhook URL
  signingSecret: ""          # default HMAC signing secret
  eventFilters: []           # default event filters
  historySync:               # default history sync (true/false)

eventsPublishVia: "webhook"  # "webhook", "redis", or "none"

redis:
  mode: "standalone"         # "standalone" or "sentinel"
  url: "localhost:6379"      # host:port or comma-separated sentinel addrs
  password: ""
  db: 0
  streamName: ""             # default: "stream:<instanceId>"
  tls: false
  tlsInsecure: false
  masterName: "mymaster"     # sentinel master name
  sentinelPassword: ""
```

## Environment Variables

All variables are optional — defaults are used when not set.

### Server

| Variable | Description | Default |
|----------|-------------|---------|
| `WSAPI_PORT` | HTTP port | `8080` |
| `WSAPI_INSTANCE_MODE` | Instance mode: `"single"` or `"multi"` | `single` |

### Database

| Variable | Description | Default |
|----------|-------------|---------|
| `WSAPI_DB_DRIVER` | Database driver (`sqlite`/`postgres`) | `sqlite` |
| `WSAPI_DB_DSN` | Database DSN | `./data/wsapi.db` |

### Authentication

| Variable | Description | Default |
|----------|-------------|---------|
| `WSAPI_ADMIN_API_KEY` | API key for `/admin/instances` (multi mode only) | |
| `WSAPI_DEFAULT_API_KEY` | Default API key for instance APIs | |

### Logging

| Variable | Description | Default |
|----------|-------------|---------|
| `WSAPI_LOG_LEVEL` | `debug`, `info`, `warn`, `error` | `info` |
| `WSAPI_LOG_FORMAT` | `text` or `json` | `text` |
| `WSAPI_LOG_REDACT` | Redact sensitive fields in event data logs | `true` |
| `WSAPI_WHATSMEOW_LOG_LEVEL` | Log level for whatsmeow | `warn` |

### WhatsApp Pairing

| Variable | Description | Default |
|----------|-------------|---------|
| `WSAPI_WHATSMEOW_PAIR_CLIENT_TYPE` | Pair client type (`chrome`, `edge`, `firefox`, `opera`, `safari`) | `chrome` |
| `WSAPI_WHATSMEOW_PAIR_CLIENT_OS` | OS name shown in WhatsApp Linked Devices (`Windows`, `Linux`, `macOS`) | `Windows` |

### Instance Defaults

| Variable | Description | Default |
|----------|-------------|---------|
| `WSAPI_DEFAULT_WEBHOOK_URL` | Default webhook URL for new instances | |
| `WSAPI_DEFAULT_SIGNING_SECRET` | Default HMAC-SHA256 signing secret | |
| `WSAPI_DEFAULT_EVENT_FILTERS` | Comma-separated event filter list | |
| `WSAPI_DEFAULT_HISTORY_SYNC` | Default history sync (`true`/`false`) | |

### Event Publishing

| Variable | Description | Default |
|----------|-------------|---------|
| `WSAPI_PUBLISH_VIA` | Event publish method: `"webhook"`, `"redis"`, or `"none"` | `webhook` |
| `WSAPI_HTTP_PROXY` | HTTP proxy URL for outbound requests | |

### Redis

| Variable | Description | Default |
|----------|-------------|---------|
| `WSAPI_REDIS_MODE` | `"standalone"` or `"sentinel"` | `standalone` |
| `WSAPI_REDIS_URL` | Host:port (standalone) or comma-separated addrs (sentinel) | |
| `WSAPI_REDIS_PASSWORD` | Redis password | |
| `WSAPI_REDIS_DB` | Redis DB number | `0` |
| `WSAPI_REDIS_STREAM_NAME` | Fixed Redis stream name | `stream:<instanceId>` |
| `WSAPI_REDIS_TLS` | Enable TLS (`true`/`false`) | `false` |
| `WSAPI_REDIS_TLS_INSECURE` | Skip TLS certificate verification | `false` |
| `WSAPI_REDIS_MASTER_NAME` | Sentinel master name | `mymaster` |
| `WSAPI_REDIS_SENTINEL_PASSWORD` | Sentinel auth password | |

## PostgreSQL Connection Strings

```bash
WSAPI_DB_DRIVER=postgres
WSAPI_DB_DSN="postgres://user:pass@localhost:5432/wsapi?sslmode=disable"
```

See [Database Setup](Database-Setup) for more details on database configuration.
