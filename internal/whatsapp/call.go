package whatsapp

import (
	"context"
	"fmt"
	"log/slog"

	"go.mau.fi/whatsmeow"
	waTypes "go.mau.fi/whatsmeow/types"
)

// CallService wraps the whatsmeow client for call operations.
type CallService struct {
	client *whatsmeow.Client
	logger *slog.Logger
}

// RejectCall rejects an incoming call.
func (c *CallService) RejectCall(ctx context.Context, callerID, callID string) error {
	if callerID == "" {
		return fmt.Errorf("callerID cannot be empty")
	}
	if callID == "" {
		return fmt.Errorf("callID cannot be empty")
	}

	callerJID, err := waTypes.ParseJID(callerID)
	if err != nil {
		return fmt.Errorf("invalid callerID: %v", err)
	}

	if err := c.client.RejectCall(ctx, callerJID, callID); err != nil {
		return fmt.Errorf("failed to reject call: %v", err)
	}
	return nil
}
