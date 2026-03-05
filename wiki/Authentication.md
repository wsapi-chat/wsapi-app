# Authentication

WSAPI uses API key authentication via the `X-Api-Key` HTTP header. There are two levels of authentication depending on the endpoint type.

## Admin Endpoints (Multi Mode Only)

Admin endpoints (`/admin/instances/*`) are authenticated with the admin API key.

| Header | Value |
|--------|-------|
| `X-Api-Key` | Value of `WSAPI_ADMIN_API_KEY` |

```bash
curl -X POST http://localhost:8080/admin/instances \
  -H "X-Api-Key: my-admin-key" \
  -H "Content-Type: application/json" \
  -d '{"id": "my-instance"}'
```

Admin endpoints are **not registered** in single mode — they return 404.

## Instance Endpoints

Instance endpoints (`/*`) handle WhatsApp operations (messages, groups, session, etc.).

### Multi Mode

Both `X-Instance-Id` and `X-Api-Key` headers are required:

| Header | Value |
|--------|-------|
| `X-Instance-Id` | The instance ID |
| `X-Api-Key` | The instance's API key, or the default API key |

```bash
curl http://localhost:8080/session/status \
  -H "X-Instance-Id: my-instance" \
  -H "X-Api-Key: my-instance-key"
```

### Single Mode

Only `X-Api-Key` is required — `X-Instance-Id` is not needed because the default instance is resolved automatically:

| Header | Value |
|--------|-------|
| `X-Api-Key` | Value of `WSAPI_DEFAULT_API_KEY` |

```bash
curl http://localhost:8080/session/status \
  -H "X-Api-Key: my-secret-key"
```

## API Key Resolution

When an instance is created (or restored on startup), empty config fields are filled from `instanceDefaults`:

1. If the instance has a custom `apiKey` set → that key is used
2. If the instance's `apiKey` is empty → `WSAPI_DEFAULT_API_KEY` is used
3. If neither is set → the endpoint is unauthenticated (not recommended)

This means you can set a single `WSAPI_DEFAULT_API_KEY` and all instances will use it unless they have a custom key.

## Unauthenticated Endpoints

The following endpoints do not require authentication:

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check |

## Summary

| Endpoint Group | Mode | Required Headers |
|----------------|------|------------------|
| `/health` | Any | None |
| `/admin/instances/*` | Multi | `X-Api-Key` (admin key) |
| `/*` (instance) | Multi | `X-Instance-Id` + `X-Api-Key` |
| `/*` (instance) | Single | `X-Api-Key` |
