FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o /build/server ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget

WORKDIR /app

COPY --from=builder /build/server /app/server
COPY config.example.yaml /app/config.yaml

RUN mkdir -p /app/data

EXPOSE 8080

ENTRYPOINT ["/app/server"]
