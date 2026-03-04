package logging

import (
	"context"
	"encoding/json"
	"log/slog"
)

const redactedPlaceholder = "[REDACTED]"

// DefaultDeepRedactKeys are top-level slog attribute keys whose JSON values
// should have sensitive fields redacted rather than being replaced entirely.
var DefaultDeepRedactKeys = []string{"eventData"}

// DefaultSensitiveFields are JSON field names within deep-redacted attributes
// whose values should be replaced with "[REDACTED]". Applied at any nesting depth.
var DefaultSensitiveFields = []string{
	"text",          // message body, quoted message text
	"caption",       // media caption
	"contact",       // VCard data
	"contactArray",  // multiple VCards
	"jpegThumbnail", // image thumbnail binary
	"filename",      // media filename
	"matchedText",   // link preview URL
	"title",         // document / link preview title
	"description",   // link preview description
	"topic",         // group description
	"pushName",      // user display name
	"fullName",      // contact full name
}

// RedactHandler wraps an slog.Handler to redact sensitive fields within
// specific log attributes. It parses JSON values of "deep redact" keys
// and replaces sensitive nested fields with "[REDACTED]".
type RedactHandler struct {
	inner      slog.Handler
	deepRedact map[string]struct{}
	fields     map[string]struct{}
}

// NewRedactHandler creates a handler that performs field-level redaction on
// the values of deepRedactKeys. Within those values, any JSON field whose
// name appears in sensitiveFields is replaced with "[REDACTED]".
func NewRedactHandler(inner slog.Handler, deepRedactKeys, sensitiveFields []string) *RedactHandler {
	dr := make(map[string]struct{}, len(deepRedactKeys))
	for _, k := range deepRedactKeys {
		dr[k] = struct{}{}
	}
	sf := make(map[string]struct{}, len(sensitiveFields))
	for _, k := range sensitiveFields {
		sf[k] = struct{}{}
	}
	return &RedactHandler{inner: inner, deepRedact: dr, fields: sf}
}

func (h *RedactHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *RedactHandler) Handle(ctx context.Context, r slog.Record) error {
	nr := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		nr.AddAttrs(h.redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, nr)
}

func (h *RedactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = h.redactAttr(a)
	}
	return &RedactHandler{
		inner:      h.inner.WithAttrs(redacted),
		deepRedact: h.deepRedact,
		fields:     h.fields,
	}
}

func (h *RedactHandler) WithGroup(name string) slog.Handler {
	return &RedactHandler{
		inner:      h.inner.WithGroup(name),
		deepRedact: h.deepRedact,
		fields:     h.fields,
	}
}

func (h *RedactHandler) redactAttr(a slog.Attr) slog.Attr {
	if _, ok := h.deepRedact[a.Key]; ok {
		return h.deepRedactAttr(a)
	}
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		redacted := make([]slog.Attr, len(attrs))
		for i, ga := range attrs {
			redacted[i] = h.redactAttr(ga)
		}
		return slog.Group(a.Key, attrsToAny(redacted)...)
	}
	return a
}

func (h *RedactHandler) deepRedactAttr(a slog.Attr) slog.Attr {
	val := a.Value.Resolve()
	underlying := val.Any()
	if underlying == nil {
		return a
	}

	var raw []byte
	switch v := underlying.(type) {
	case json.RawMessage:
		raw = v
	case []byte:
		if json.Valid(v) {
			raw = v
		}
	default:
		var err error
		raw, err = json.Marshal(v)
		if err != nil {
			return slog.String(a.Key, redactedPlaceholder)
		}
	}
	if raw == nil {
		return slog.String(a.Key, redactedPlaceholder)
	}

	redacted := RedactJSON(raw, h.fields)
	return slog.Any(a.Key, json.RawMessage(redacted))
}

// RedactJSON parses data as JSON. For any object key that appears in fields,
// the value is replaced with "[REDACTED]". Arrays and nested objects are
// walked recursively.
func RedactJSON(data []byte, fields map[string]struct{}) []byte {
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return []byte(`"` + redactedPlaceholder + `"`)
	}
	redactValue(parsed, fields)
	out, err := json.Marshal(parsed)
	if err != nil {
		return []byte(`"` + redactedPlaceholder + `"`)
	}
	return out
}

func redactValue(v any, fields map[string]struct{}) {
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			if _, ok := fields[k]; ok {
				val[k] = redactedPlaceholder
			} else {
				redactValue(child, fields)
			}
		}
	case []any:
		for _, item := range val {
			redactValue(item, fields)
		}
	}
}

func attrsToAny(attrs []slog.Attr) []any {
	out := make([]any, len(attrs))
	for i, a := range attrs {
		out[i] = a
	}
	return out
}
