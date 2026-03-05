# Getting Started

## Running the Server

### Docker Hub

```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -e WSAPI_DEFAULT_API_KEY=my-secret-key \
  wsapichat/wsapi:latest
```

### Docker Compose

```bash
git clone https://github.com/wsapi-chat/wsapi-app.git
cd wsapi-app
cp config.example.yaml config.yaml   # edit as needed
docker compose up --build
```

### From Source (requires Go 1.25+)

```bash
git clone https://github.com/wsapi-chat/wsapi-app.git
cd wsapi-app
cp config.example.yaml config.yaml   # edit as needed
go build -o bin/server ./cmd/server
WSAPI_DEFAULT_API_KEY=my-secret-key ./bin/server
```

The server starts on port `8080` with SQLite storage in `./data/`. Verify it's running:

```bash
curl http://localhost:8080/health
# Returns: {"status":"ok"}
```

## Quick Start (Single Mode)

Single mode is the default — a single `"default"` instance is used, so there is no need for instance management or an app store. No `X-Instance-Id` header is needed, and the admin API is not available. See [Instance Modes](Instance-Modes) for details.

### 1. Pair your device

```bash
curl http://localhost:8080/session/qr \
  -H "X-Api-Key: my-secret-key" \
  --output qr.png
# Open qr.png and scan with WhatsApp > Linked Devices > Link a Device
```

### 2. Verify the connection

```bash
curl http://localhost:8080/session/status \
  -H "X-Api-Key: my-secret-key"
# Returns: {"isConnected": true, "isLoggedIn": true, "deviceId": "..."}
```

### 3. Send a message

```bash
curl -X POST http://localhost:8080/messages/text \
  -H "X-Api-Key: my-secret-key" \
  -H "Content-Type: application/json" \
  -d '{"to": "1234567890", "text": "Hello from WSAPI!"}'
```

## Quick Start (Multi Mode)

Multi mode lets you run multiple WhatsApp sessions behind a single server. Each session is an **instance** managed via the admin API.

### 1. Start the server

```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -e WSAPI_INSTANCE_MODE=multi \
  -e WSAPI_ADMIN_API_KEY=my-admin-key \
  -e WSAPI_DEFAULT_API_KEY=my-instance-key \
  wsapichat/wsapi:latest
```

### 2. Create an instance

```bash
curl -X POST http://localhost:8080/admin/instances \
  -H "X-Api-Key: my-admin-key" \
  -H "Content-Type: application/json" \
  -d '{"id": "my-instance"}'
```

### 3. Pair your device

```bash
curl http://localhost:8080/session/qr \
  -H "X-Instance-Id: my-instance" \
  -H "X-Api-Key: my-instance-key" \
  --output qr.png
# Open qr.png and scan with WhatsApp > Linked Devices > Link a Device
```

### 4. Send a message

```bash
curl -X POST http://localhost:8080/messages/text \
  -H "X-Instance-Id: my-instance" \
  -H "X-Api-Key: my-instance-key" \
  -H "Content-Type: application/json" \
  -d '{"to": "1234567890", "text": "Hello from WSAPI!"}'
```

## Next Steps

- [Configure webhooks and event delivery](Event-Delivery)
- [Set up PostgreSQL for production](Database-Setup)
- [Deploy with Docker](Docker-Deployment)
- [Browse all API endpoints](API-Overview)
