package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func newTestLogger(buf *bytes.Buffer, handler *RedactHandler) *slog.Logger {
	return slog.New(handler)
}

func newRedactHandler(buf *bytes.Buffer) *RedactHandler {
	inner := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return NewRedactHandler(inner, DefaultDeepRedactKeys, DefaultSensitiveFields)
}

func parseLogLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("failed to parse log line: %v\nraw: %s", err, buf.String())
	}
	return m
}

func TestRedactHandler_FieldLevelRedaction(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newTestLogger(buf, newRedactHandler(buf))

	eventData := json.RawMessage(`{
		"id": "MSG123",
		"chatId": "1234567890@s.whatsapp.net",
		"text": "Hello, world!",
		"type": "text",
		"isGroup": false
	}`)

	logger.Debug("publishing event",
		"eventType", "message",
		"eventData", eventData,
	)

	m := parseLogLine(t, buf)

	ed, ok := m["eventData"].(map[string]any)
	if !ok {
		t.Fatalf("eventData is not an object: %T", m["eventData"])
	}

	// text should be redacted
	if ed["text"] != redactedPlaceholder {
		t.Errorf("text = %q, want %q", ed["text"], redactedPlaceholder)
	}

	// structural fields should be preserved
	if ed["id"] != "MSG123" {
		t.Errorf("id = %q, want %q", ed["id"], "MSG123")
	}
	if ed["chatId"] != "1234567890@s.whatsapp.net" {
		t.Errorf("chatId = %q, want %q", ed["chatId"], "1234567890@s.whatsapp.net")
	}
	if ed["type"] != "text" {
		t.Errorf("type = %q, want %q", ed["type"], "text")
	}
	if ed["isGroup"] != false {
		t.Errorf("isGroup = %v, want false", ed["isGroup"])
	}
}

func TestRedactHandler_NestedObjectRedaction(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newTestLogger(buf, newRedactHandler(buf))

	eventData := json.RawMessage(`{
		"id": "MSG456",
		"chatId": "group@g.us",
		"media": {
			"mediaType": "image",
			"mimeType": "image/jpeg",
			"caption": "Check this out!",
			"filename": "photo.jpg",
			"jpegThumbnail": "base64data",
			"fileLength": 12345
		},
		"replyTo": {
			"id": "MSG100",
			"text": "Original message"
		}
	}`)

	logger.Debug("event", "eventData", eventData)

	m := parseLogLine(t, buf)
	ed := m["eventData"].(map[string]any)
	media := ed["media"].(map[string]any)
	reply := ed["replyTo"].(map[string]any)

	// media sensitive fields redacted
	if media["caption"] != redactedPlaceholder {
		t.Errorf("media.caption = %q, want redacted", media["caption"])
	}
	if media["filename"] != redactedPlaceholder {
		t.Errorf("media.filename = %q, want redacted", media["filename"])
	}
	if media["jpegThumbnail"] != redactedPlaceholder {
		t.Errorf("media.jpegThumbnail = %q, want redacted", media["jpegThumbnail"])
	}

	// media structural fields preserved
	if media["mediaType"] != "image" {
		t.Errorf("media.mediaType = %q, want %q", media["mediaType"], "image")
	}
	if media["mimeType"] != "image/jpeg" {
		t.Errorf("media.mimeType = %q, want %q", media["mimeType"], "image/jpeg")
	}
	if media["fileLength"] != float64(12345) {
		t.Errorf("media.fileLength = %v, want 12345", media["fileLength"])
	}

	// replyTo.text redacted, replyTo.id preserved
	if reply["text"] != redactedPlaceholder {
		t.Errorf("replyTo.text = %q, want redacted", reply["text"])
	}
	if reply["id"] != "MSG100" {
		t.Errorf("replyTo.id = %q, want %q", reply["id"], "MSG100")
	}
}

func TestRedactHandler_ArrayRecursion(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newTestLogger(buf, newRedactHandler(buf))

	eventData := json.RawMessage(`{
		"chatId": "1234567890@s.whatsapp.net",
		"messages": [
			{"id": "MSG1", "text": "First message", "type": "text"},
			{"id": "MSG2", "text": "Second message", "type": "text", "media": {"caption": "A caption"}}
		]
	}`)

	logger.Debug("event", "eventData", eventData)

	m := parseLogLine(t, buf)
	ed := m["eventData"].(map[string]any)
	msgs := ed["messages"].([]any)

	msg1 := msgs[0].(map[string]any)
	if msg1["text"] != redactedPlaceholder {
		t.Errorf("messages[0].text = %q, want redacted", msg1["text"])
	}
	if msg1["id"] != "MSG1" {
		t.Errorf("messages[0].id = %q, want %q", msg1["id"], "MSG1")
	}

	msg2 := msgs[1].(map[string]any)
	if msg2["text"] != redactedPlaceholder {
		t.Errorf("messages[1].text = %q, want redacted", msg2["text"])
	}
	media := msg2["media"].(map[string]any)
	if media["caption"] != redactedPlaceholder {
		t.Errorf("messages[1].media.caption = %q, want redacted", media["caption"])
	}
}

func TestRedactHandler_NonRedactedKeysPassThrough(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newTestLogger(buf, newRedactHandler(buf))

	logger.Debug("test",
		"eventType", "message",
		"eventId", "evt_abc123",
		"instanceId", "inst_1",
	)

	m := parseLogLine(t, buf)

	if m["eventType"] != "message" {
		t.Errorf("eventType = %q, want %q", m["eventType"], "message")
	}
	if m["eventId"] != "evt_abc123" {
		t.Errorf("eventId = %q, want %q", m["eventId"], "evt_abc123")
	}
	if m["instanceId"] != "inst_1" {
		t.Errorf("instanceId = %q, want %q", m["instanceId"], "inst_1")
	}
}

func TestRedactHandler_WithAttrsDeepRedact(t *testing.T) {
	buf := &bytes.Buffer{}
	h := newRedactHandler(buf)
	logger := slog.New(h)

	eventData := json.RawMessage(`{"text": "secret", "chatId": "123@s.whatsapp.net"}`)
	child := logger.With("eventData", eventData)

	child.Debug("test")

	m := parseLogLine(t, buf)
	ed := m["eventData"].(map[string]any)

	if ed["text"] != redactedPlaceholder {
		t.Errorf("WithAttrs text = %q, want redacted", ed["text"])
	}
	if ed["chatId"] != "123@s.whatsapp.net" {
		t.Errorf("WithAttrs chatId = %q, want preserved", ed["chatId"])
	}
}

func TestRedactHandler_GoStructValue(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newTestLogger(buf, newRedactHandler(buf))

	type testEvent struct {
		ID     string `json:"id"`
		ChatID string `json:"chatId"`
		Text   string `json:"text"`
	}

	logger.Debug("event", "eventData", testEvent{
		ID:     "MSG789",
		ChatID: "5551234@s.whatsapp.net",
		Text:   "Secret message",
	})

	m := parseLogLine(t, buf)
	ed := m["eventData"].(map[string]any)

	if ed["text"] != redactedPlaceholder {
		t.Errorf("struct text = %q, want redacted", ed["text"])
	}
	if ed["id"] != "MSG789" {
		t.Errorf("struct id = %q, want %q", ed["id"], "MSG789")
	}
	if ed["chatId"] != "5551234@s.whatsapp.net" {
		t.Errorf("struct chatId = %q, want %q", ed["chatId"], "5551234@s.whatsapp.net")
	}
}

func TestRedactHandler_NilValue(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newTestLogger(buf, newRedactHandler(buf))

	logger.Debug("event", "eventData", nil, "other", "value")

	m := parseLogLine(t, buf)
	if m["other"] != "value" {
		t.Errorf("other = %q, want %q", m["other"], "value")
	}
}

func TestRedactHandler_InvalidJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newTestLogger(buf, newRedactHandler(buf))

	logger.Debug("event", "eventData", []byte("not json {{{"))

	m := parseLogLine(t, buf)
	if m["eventData"] != redactedPlaceholder {
		t.Errorf("invalid JSON eventData = %q, want %q", m["eventData"], redactedPlaceholder)
	}
}

func TestRedactHandler_EmptyKeys(t *testing.T) {
	buf := &bytes.Buffer{}
	inner := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := NewRedactHandler(inner, nil, nil)
	logger := slog.New(h)

	eventData := json.RawMessage(`{"text": "visible", "chatId": "123@s.whatsapp.net"}`)
	logger.Debug("event", "eventData", eventData)

	m := parseLogLine(t, buf)
	ed := m["eventData"].(map[string]any)

	// With no sensitive fields, nothing should be redacted
	if ed["text"] != "visible" {
		t.Errorf("empty keys: text = %q, want %q", ed["text"], "visible")
	}
}

func TestRedactHandler_Enabled(t *testing.T) {
	buf := &bytes.Buffer{}
	inner := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	h := NewRedactHandler(inner, DefaultDeepRedactKeys, DefaultSensitiveFields)

	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("Enabled(Debug) should be false when inner handler level is Info")
	}
	if !h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("Enabled(Info) should be true when inner handler level is Info")
	}
}

func TestRedactHandler_AllSensitiveFields(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newTestLogger(buf, newRedactHandler(buf))

	eventData := json.RawMessage(`{
		"id": "MSG",
		"text": "msg text",
		"media": {
			"caption": "cap",
			"filename": "file.jpg",
			"jpegThumbnail": "thumb",
			"title": "doc title"
		},
		"contact": "BEGIN:VCARD",
		"contactArray": ["VCARD1", "VCARD2"],
		"extendedText": {
			"matchedText": "https://example.com",
			"title": "Link Title",
			"description": "Link desc"
		},
		"sender": {"id": "123@s.whatsapp.net", "phone": "123"}
	}`)

	logger.Debug("event", "eventData", eventData)

	m := parseLogLine(t, buf)
	ed := m["eventData"].(map[string]any)

	// All sensitive fields should be redacted
	for _, key := range []string{"text", "contact"} {
		if ed[key] != redactedPlaceholder {
			t.Errorf("%s = %q, want redacted", key, ed[key])
		}
	}

	// contactArray redacted (entire value replaced)
	if ed["contactArray"] != redactedPlaceholder {
		t.Errorf("contactArray = %v, want redacted", ed["contactArray"])
	}

	// Nested media fields
	media := ed["media"].(map[string]any)
	for _, key := range []string{"caption", "filename", "jpegThumbnail", "title"} {
		if media[key] != redactedPlaceholder {
			t.Errorf("media.%s = %q, want redacted", key, media[key])
		}
	}

	// Nested extendedText fields
	ext := ed["extendedText"].(map[string]any)
	for _, key := range []string{"matchedText", "title", "description"} {
		if ext[key] != redactedPlaceholder {
			t.Errorf("extendedText.%s = %q, want redacted", key, ext[key])
		}
	}

	// Sender should NOT be redacted
	sender := ed["sender"].(map[string]any)
	if sender["id"] != "123@s.whatsapp.net" {
		t.Errorf("sender.id = %q, want preserved", sender["id"])
	}
	if sender["phone"] != "123" {
		t.Errorf("sender.phone = %q, want preserved", sender["phone"])
	}

	// ID should NOT be redacted
	if ed["id"] != "MSG" {
		t.Errorf("id = %q, want preserved", ed["id"])
	}
}

func TestRedactHandler_GroupInfoRedaction(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newTestLogger(buf, newRedactHandler(buf))

	eventData := json.RawMessage(`{
		"id": "GRP123",
		"description": {"topic": "Secret group topic"},
		"name": {"name": "Group Name"}
	}`)

	logger.Debug("event", "eventData", eventData)

	m := parseLogLine(t, buf)
	ed := m["eventData"].(map[string]any)

	// "description" key is in sensitive fields, so the entire value is replaced
	if ed["description"] != redactedPlaceholder {
		t.Errorf("description = %v, want redacted", ed["description"])
	}

	// "id" should be preserved
	if ed["id"] != "GRP123" {
		t.Errorf("id = %q, want %q", ed["id"], "GRP123")
	}
}

func TestRedactHandler_PushNameAndFullName(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newTestLogger(buf, newRedactHandler(buf))

	// ChatPushNameEvent
	eventData := json.RawMessage(`{
		"user": {"id": "123@s.whatsapp.net", "phone": "123"},
		"pushName": "John Doe"
	}`)
	logger.Debug("event", "eventData", eventData)

	m := parseLogLine(t, buf)
	ed := m["eventData"].(map[string]any)

	if ed["pushName"] != redactedPlaceholder {
		t.Errorf("pushName = %q, want redacted", ed["pushName"])
	}
	user := ed["user"].(map[string]any)
	if user["id"] != "123@s.whatsapp.net" {
		t.Errorf("user.id = %q, want preserved", user["id"])
	}

	// ContactEvent
	buf.Reset()
	eventData = json.RawMessage(`{
		"contact": {"id": "456@s.whatsapp.net"},
		"fullName": "Jane Smith"
	}`)
	logger.Debug("event", "eventData", eventData)

	m = parseLogLine(t, buf)
	ed = m["eventData"].(map[string]any)

	if ed["fullName"] != redactedPlaceholder {
		t.Errorf("fullName = %q, want redacted", ed["fullName"])
	}
}

func TestRedactJSON(t *testing.T) {
	fields := map[string]struct{}{
		"text":    {},
		"caption": {},
	}

	input := []byte(`{"id":"1","text":"hello","media":{"caption":"cap","size":100}}`)
	result := RedactJSON(input, fields)

	var m map[string]any
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatalf("failed to parse redacted JSON: %v", err)
	}

	if m["id"] != "1" {
		t.Errorf("id = %q, want %q", m["id"], "1")
	}
	if m["text"] != redactedPlaceholder {
		t.Errorf("text = %q, want redacted", m["text"])
	}

	media := m["media"].(map[string]any)
	if media["caption"] != redactedPlaceholder {
		t.Errorf("media.caption = %q, want redacted", media["caption"])
	}
	if media["size"] != float64(100) {
		t.Errorf("media.size = %v, want 100", media["size"])
	}
}
