# Docker Deployment

WSAPI is available as a Docker image on Docker Hub at [`wsapichat/wsapi`](https://hub.docker.com/r/wsapichat/wsapi).

## Docker Run

### Minimal (Single Mode)

```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -e WSAPI_DEFAULT_API_KEY=my-secret-key \
  wsapichat/wsapi:latest
```

### Multi Mode

```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -e WSAPI_INSTANCE_MODE=multi \
  -e WSAPI_ADMIN_API_KEY=my-admin-key \
  -e WSAPI_DEFAULT_API_KEY=my-instance-key \
  -e WSAPI_PUBLISH_VIA=webhook \
  -e WSAPI_DEFAULT_WEBHOOK_URL=https://example.com/webhook \
  wsapichat/wsapi:latest
```

### With Config File

```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  wsapichat/wsapi:latest
```

## Docker Compose

Create a `docker-compose.yml`:

```yaml
services:
  wsapi:
    build: .                          # or use image: wsapichat/wsapi:latest
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data              # SQLite databases
      - ./config.yaml:/app/config.yaml:ro  # config file (optional)
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "-O", "/dev/null", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
```

Start the stack:

```bash
docker compose up -d
```

## Volumes

| Container Path | Purpose |
|----------------|---------|
| `/app/data` | SQLite database (`wsapi.db`) — device sessions, instance records, chats, contacts |
| `/app/config.yaml` | Configuration file (mount as read-only with `:ro`) |

**Important:** Always mount `/app/data` as a volume to persist your WhatsApp sessions and databases across container restarts.

## Health Check

The built-in health check hits `GET /health`:

```bash
curl http://localhost:8080/health
# Returns: {"status":"ok"}
```

The Docker Compose example includes an automated health check that runs every 30 seconds.

## Environment Variables

Pass environment variables with `-e` flags or an env file:

```bash
# Using -e flags
docker run -d \
  -e WSAPI_ADMIN_API_KEY=my-key \
  -e WSAPI_PUBLISH_VIA=webhook \
  wsapichat/wsapi:latest

# Using an env file
docker run -d --env-file .env wsapichat/wsapi:latest
```

See [Configuration](Configuration) for the full list of environment variables.

## Production Tips

### Use a specific image tag

Pin to a specific version instead of `latest` to avoid unexpected upgrades:

```bash
docker run -d wsapichat/wsapi:v2.1.0
```

### Set API keys

Always set `WSAPI_ADMIN_API_KEY` and `WSAPI_DEFAULT_API_KEY` in production to protect your endpoints.

### Use PostgreSQL for production

SQLite is great for development, but PostgreSQL is recommended for production deployments with multiple instances. See [Database Setup](Database-Setup).

### Use JSON logging

Set `WSAPI_LOG_FORMAT=json` for structured logging that integrates with log aggregation tools.

### Proxy configuration

If your server needs an HTTP proxy for outbound requests (webhook delivery, media downloads):

```bash
docker run -d \
  -e WSAPI_HTTP_PROXY=http://proxy.example.com:8080 \
  wsapichat/wsapi:latest
```

### Resource limits

Set memory and CPU limits appropriate for your workload:

```yaml
services:
  wsapi:
    image: wsapichat/wsapi:latest
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: "1.0"
```
