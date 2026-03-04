package whatsapp

import (
	"context"
	"encoding/json"
	"log/slog"

	"go.mau.fi/whatsmeow"

	"github.com/wsapi-chat/wsapi-app/internal/event"
	"github.com/wsapi-chat/wsapi-app/internal/publisher"
)

const historySyncChunkSize = 500

// HistorySyncService provides the flush-history-sync endpoint logic.
type HistorySyncService struct {
	client *whatsmeow.Client
	logger *slog.Logger
	store  *HistorySyncStore
}

// FlushHistory reads all cached history sync messages from the DB, publishes
// them as message_history_sync events (one per chat, chunked at 500 messages),
// and deletes the cache. It is intended to run asynchronously in a goroutine.
func (s *HistorySyncService) FlushHistory(ctx context.Context, pub publisher.Publisher, instanceID string) {
	if s.client.Store.ID == nil {
		s.logger.Warn("flush history: device not paired")
		return
	}
	ourJID := s.client.Store.ID.String()

	chats, err := s.store.ListChats(ctx, ourJID)
	if err != nil {
		s.logger.Error("flush history: list chats", "error", err)
		return
	}

	if len(chats) == 0 {
		s.logger.Info("flush history: no cached messages")
		return
	}

	s.logger.Info("flush history: starting", "chats", len(chats))

	for _, chatJID := range chats {
		jsonArrays, err := s.store.GetMessages(ctx, ourJID, chatJID)
		if err != nil {
			s.logger.Error("flush history: get messages", "chatJid", chatJID, "error", err)
			continue
		}

		// Deserialize all JSON arrays and collect messages with deduplication.
		seen := make(map[string]struct{})
		var buffer []event.MessageEvent

		for _, jsonStr := range jsonArrays {
			var msgs []event.MessageEvent
			if err := json.Unmarshal([]byte(jsonStr), &msgs); err != nil {
				s.logger.Error("flush history: unmarshal messages", "chatJid", chatJID, "error", err)
				continue
			}

			for _, msg := range msgs {
				if _, dup := seen[msg.ID]; dup {
					continue
				}
				seen[msg.ID] = struct{}{}
				buffer = append(buffer, msg)

				if len(buffer) >= historySyncChunkSize {
					s.publishChunk(ctx, pub, instanceID, chatJID, buffer)
					buffer = nil
				}
			}
		}

		// Publish remaining messages for this chat.
		if len(buffer) > 0 {
			s.publishChunk(ctx, pub, instanceID, chatJID, buffer)
		}
	}

	// Clean up all cached rows.
	if err := s.store.DeleteAll(ctx, ourJID); err != nil {
		s.logger.Error("flush history: delete cache", "error", err)
	}

	s.logger.Info("flush history: completed")
}

func (s *HistorySyncService) publishChunk(ctx context.Context, pub publisher.Publisher, instanceID, chatJID string, messages []event.MessageEvent) {
	evt := event.NewEvent(instanceID, event.TypeMessageHistorySync, event.HistorySyncEvent{
		ChatID:   chatJID,
		Messages: messages,
	})

	if err := pub.Publish(ctx, evt); err != nil {
		s.logger.Error("flush history: publish chunk",
			"chatJid", chatJID,
			"messages", len(messages),
			"error", err,
		)
	}
}
