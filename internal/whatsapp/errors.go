package whatsapp

import (
	"errors"
	"time"

	"go.mau.fi/whatsmeow"
)

// SendAckTimeout is whatsmeow's defaultRequestTimeout: how long SendMessage
// waits for the server to acknowledge a message before giving up. It is an
// unexported constant upstream, mirrored here because the HTTP server's write
// deadline has to outlast it — see ServerConfig.WriteTimeoutDuration.
const SendAckTimeout = 75 * time.Second

// ErrNotFound indicates the requested resource was not found.
var ErrNotFound = errors.New("not found")

// ErrUpstream indicates the WhatsApp server returned an error.
var ErrUpstream = errors.New("upstream error")

// ErrTooLarge indicates the requested media file exceeds the configured size limit.
var ErrTooLarge = errors.New("file too large")

// ErrTimeout indicates a service-side wait exceeded its budget. Distinct from
// context.Canceled (caller went away): this is the server giving up.
var ErrTimeout = errors.New("timeout")

// IsUpstreamTimeout reports whether err is whatsmeow giving up waiting on
// WhatsApp. Two sentinels mean this, and a send can hit either:
//
//   - ErrMessageTimedOut — the message went out, no acknowledgement came back.
//   - ErrIQTimedOut — an info query stalled. Sends make these too, to fetch
//     prekeys or sessions for a recipient they have no session with yet.
//
// Both are bare errors.New values (whatsmeow/errors.go:21,24) wrapping neither
// context.DeadlineExceeded nor anything else, so callers matching on the usual
// timeout sentinels miss them entirely and fall through to whatever their
// default case is. Keeping the check here means the handler layer maps them
// without importing whatsmeow.
//
// Both share the same 75s budget (defaultRequestTimeout), so neither is
// distinguishable from the other by duration alone — only by the error text,
// which is why ServiceError logs it.
func IsUpstreamTimeout(err error) bool {
	return errors.Is(err, whatsmeow.ErrMessageTimedOut) || errors.Is(err, whatsmeow.ErrIQTimedOut)
}
