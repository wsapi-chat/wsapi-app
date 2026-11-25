package event

import (
	"testing"
	"time"

	waSyncAction "go.mau.fi/whatsmeow/proto/waSyncAction"
	waTypes "go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func testChatJID(t *testing.T) waTypes.JID {
	t.Helper()
	jid, err := waTypes.ParseJID("15550000000@s.whatsapp.net")
	if err != nil {
		t.Fatal(err)
	}
	return jid
}

func TestProjectMessageDeleteForMe_PublishesLiveDeletion(t *testing.T) {
	evt := &waEvents.DeleteForMe{
		ChatJID:   testChatJID(t),
		MessageID: "3EB0ABC123",
		IsFromMe:  true,
		Timestamp: time.Unix(1750000000, 0),
	}

	_, data, publish := ProjectMessageDeleteForMe(evt, nil)
	if !publish {
		t.Fatal("expected live delete-for-me to be published")
	}
	result, ok := data.(MessageDeleteEvent)
	if !ok {
		t.Fatalf("expected MessageDeleteEvent, got %T", data)
	}
	if result.ID != "3EB0ABC123" || !result.IsDeletedForMe {
		t.Fatalf("unexpected event: %+v", result)
	}
}

func TestProjectMessageDeleteForMe_DiscardsFullSync(t *testing.T) {
	evt := &waEvents.DeleteForMe{
		ChatJID:      testChatJID(t),
		MessageID:    "3EB0ABC123",
		Timestamp:    time.Unix(1750000000, 0),
		FromFullSync: true,
	}

	if _, _, publish := ProjectMessageDeleteForMe(evt, nil); publish {
		t.Fatal("expected full-sync delete-for-me to be discarded")
	}
}

func TestProjectStar_PublishesLiveStar(t *testing.T) {
	evt := &waEvents.Star{
		ChatJID:   testChatJID(t),
		MessageID: "3EB0ABC123",
		IsFromMe:  true,
		Timestamp: time.Unix(1750000000, 0),
		Action:    &waSyncAction.StarAction{Starred: proto.Bool(true)},
	}

	if _, _, publish := ProjectStar(evt, nil); !publish {
		t.Fatal("expected live star to be published")
	}
}

func TestProjectStar_DiscardsFullSync(t *testing.T) {
	evt := &waEvents.Star{
		ChatJID:      testChatJID(t),
		MessageID:    "3EB0ABC123",
		Timestamp:    time.Unix(1750000000, 0),
		Action:       &waSyncAction.StarAction{Starred: proto.Bool(true)},
		FromFullSync: true,
	}

	if _, _, publish := ProjectStar(evt, nil); publish {
		t.Fatal("expected full-sync star to be discarded")
	}
}
