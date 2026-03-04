FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN GOOS=linux go build -o /build/server ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget

WORKDIR /app

COPY --from=builder /build/server /app/server

RUN mkdir -p /app/data

EXPOSE 8080

ENTRYPOINT ["/app/server"]
