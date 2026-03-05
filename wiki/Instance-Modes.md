# Instance Modes

WSAPI supports two instance modes, controlled by `WSAPI_INSTANCE_MODE` (or `instanceMode` in config.yaml). The default is `single`.

## Comparison

| Feature | Single Mode | Multi Mode |
|---------|-------------|------------|
| Instance count | 1 | Unlimited (managed via admin API) |
| `X-Instance-Id` header | Not needed | Required on all instance endpoints |
| Admin API (`/admin/instances/*`) | Not registered (404) | Available |
| App store database (`WSAPI_DB_*`) | Not used | Required |
| Config source | Environment variables / config.yaml | Admin API + defaults from env/yaml |

## Single Mode

Single mode is the simplest way to run WSAPI — one WhatsApp session with no instance management overhead.

**How it works:**

1. On startup, the manager calls `EnsureSingleInstance()` which sets up a fixed `"default"` instance — since there is only one instance, the app store (`wsapi.db`) is not needed and is never created
2. The device session is resolved directly from the whatsmeow DB via `GetFirstDevice()`, so your WhatsApp session survives restarts
3. Instance config (API key, webhook URL, etc.) always comes from `instanceDefaults` in your config.yaml or environment variables
4. The `X-Instance-Id` header is not required on API requests since there is only one instance and ignored if provided

**Example request:**

```bash
curl http://localhost:8080/session/status \
  -H "X-Api-Key: my-secret-key"
```

**Applicable environment variables:**

- `WSAPI_DEFAULT_API_KEY` — API key for the default instance
- `WSAPI_DEFAULT_WEBHOOK_URL` — webhook URL
- `WSAPI_DEFAULT_SIGNING_SECRET` — HMAC signing secret
- `WSAPI_DEFAULT_EVENT_FILTERS` — comma-separated event filter list
- `WSAPI_DEFAULT_HISTORY_SYNC` — enable history sync (`true`/`false`)
- `WSAPI_WHATSMEOW_DB_DRIVER` / `WSAPI_WHATSMEOW_DB_DSN` — whatsmeow database

Variables **not used** in single mode: `WSAPI_DB_DRIVER`, `WSAPI_DB_DSN`, `WSAPI_ADMIN_API_KEY`.

## Multi Mode

Multi mode lets you run multiple independent WhatsApp sessions behind a single WSAPI server.

**How it works:**

1. On startup, the manager calls `RestoreInstances()` to load all persisted instances from the wsapi store
2. Instances are created, updated, and deleted via the admin API (`/admin/instances/*`)
3. Each instance has its own device session, API key, webhook URL, signing secret, event filters, and history sync setting
4. The `InstanceAuth` middleware resolves `X-Instance-Id` to the correct instance and validates `X-Api-Key`
5. Global defaults from `instanceDefaults` are applied when creating or restoring instances (empty fields are filled from defaults)

**Enable multi mode:**

```bash
WSAPI_INSTANCE_MODE=multi \
WSAPI_ADMIN_API_KEY=my-admin-key \
WSAPI_DEFAULT_API_KEY=my-default-key \
  ./bin/server
```

**Create an instance:**

```bash
curl -X POST http://localhost:8080/admin/instances \
  -H "X-Api-Key: my-admin-key" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "instance-1",
    "apiKey": "custom-key",
    "webhookUrl": "https://example.com/webhook"
  }'
```

**Use instance endpoints:**

```bash
curl http://localhost:8080/session/status \
  -H "X-Instance-Id: instance-1" \
  -H "X-Api-Key: custom-key"
```

**Applicable environment variables (in addition to single mode vars):**

- `WSAPI_DB_DRIVER` / `WSAPI_DB_DSN` — app store database for persisting instance records
- `WSAPI_ADMIN_API_KEY` — API key for admin endpoints
