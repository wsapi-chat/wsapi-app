package publisher

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wsapi-chat/wsapi-app/internal/config"
	"github.com/wsapi-chat/wsapi-app/internal/event"
)

// RedisPublisher publishes events to a Redis stream via XADD.
type RedisPublisher struct {
	client     *redis.Client
	streamName string
	logger     *slog.Logger
}

// NewRedisPublisher creates a shared Redis publisher. The client connects
// lazily — if Redis is temporarily unavailable at startup, individual publish
// calls will fail and be logged, but will succeed once Redis recovers.
func NewRedisPublisher(cfg *config.RedisConfig, logger *slog.Logger) *RedisPublisher {
	var tlsCfg *tls.Config
	if cfg.TLS {
		tlsCfg = &tls.Config{
			InsecureSkipVerify: cfg.TLSInsecure,
		}
	}

	maxRetries := cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = 50
	}

	var rdb *redis.Client

	switch strings.ToLower(cfg.Mode) {
	case "sentinel":
		masterName := cfg.MasterName
		if masterName == "" {
			masterName = "mymaster"
		}
		addrs := strings.Split(cfg.URL, ",")
		for i := range addrs {
			addrs[i] = strings.TrimSpace(addrs[i])
		}

		rdb = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:       masterName,
			SentinelAddrs:    addrs,
			SentinelPassword: cfg.SentinelPassword,
			DB:               cfg.DB,
			Password:         cfg.Password,
			TLSConfig:        tlsCfg,
			DialTimeout:      5 * time.Second,
			ReadTimeout:      3 * time.Second,
			WriteTimeout:     3 * time.Second,
			PoolSize:         10,
			MinIdleConns:     2,
			MaxRetries:       maxRetries,
		})
		logger.Info("Redis client configured for Sentinel mode", "masterName", masterName, "sentinelAddrs", addrs)

	default:
		rdb = redis.NewClient(&redis.Options{
			Addr:         cfg.URL,
			Password:     cfg.Password,
			DB:           cfg.DB,
			TLSConfig:    tlsCfg,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			PoolSize:     10,
			MinIdleConns: 2,
			MaxRetries:   maxRetries,
		})
		logger.Info("Redis client configured for Standalone mode", "addr", cfg.URL, "db", cfg.DB, "tls", cfg.TLS)
	}

	return &RedisPublisher{
		client:     rdb,
		streamName: cfg.StreamName,
		logger:     logger.With("publisher", "redis"),
	}
}

func (p *RedisPublisher) publish(ctx context.Context, evt event.Event, instanceID, signingSecret string) error {
	dataJSON, err := json.Marshal(evt.Data)
	if err != nil {
		return fmt.Errorf("marshal eventData: %w", err)
	}

	p.logger.Debug("publishing event",
		"eventType", evt.EventType,
		"eventId", evt.EventID,
		"instanceId", instanceID,
		"signed", signingSecret != "",
		"eventData", json.RawMessage(dataJSON),
	)

	streamName := p.streamName
	if streamName == "" {
		streamName = "stream:" + instanceID
	}
	values := map[string]interface{}{
		"eventId":    evt.EventID,
		"instanceId": instanceID,
		"eventType":  evt.EventType,
		"receivedAt": evt.ReceivedAt.Format(time.RFC3339Nano),
		"eventData":  string(dataJSON),
	}

	if signingSecret != "" {
		body, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("marshal event for signing: %w", err)
		}
		values["signature"] = Sign(body, signingSecret)
	}

	_, err = p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: values,
	}).Result()

	return err
}

func (p *RedisPublisher) Close() error {
	return p.client.Close()
}

// redisWrapper adapts the shared RedisPublisher for a specific instance.
type redisWrapper struct {
	rp         *RedisPublisher
	instanceID string
	signature  string
}

func (w *redisWrapper) Publish(ctx context.Context, evt event.Event) error {
	return w.rp.publish(ctx, evt, w.instanceID, w.signature)
}

func (w *redisWrapper) Close() error { return nil }
