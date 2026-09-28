package event

import (
	"testing"
	"time"

	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	waTypes "go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// newMessageEvent builds a minimal inbound whatsmeow Message event carrying msg.
func newMessageEvent(msg *waE2E.Message) *waEvents.Message {
	chat, _ := waTypes.ParseJID("5491100000000@s.whatsapp.net")
	return &waEvents.Message{
		Info: waTypes.MessageInfo{
			ID:        "3EB0ABC123",
			Timestamp: time.Unix(1750000000, 0),
			MessageSource: waTypes.MessageSource{
				Chat:   chat,
				Sender: chat,
			},
		},
		Message: msg,
	}
}

// ctwaAdReply is the externalAdReply a first message from a Click-to-WhatsApp ad carries.
func ctwaAdReply() *waE2E.ContextInfo_ExternalAdReplyInfo {
	return &waE2E.ContextInfo_ExternalAdReplyInfo{
		Title:             proto.String("Zapatillas 2x1"),
		Body:              proto.String("Solo por hoy"),
		MediaType:         waE2E.ContextInfo_ExternalAdReplyInfo_IMAGE.Enum(),
		ThumbnailURL:      proto.String("https://scontent.example/thumb.jpg"),
		SourceType:        proto.String("ad"),
		SourceID:          proto.String("120210000000000000"),
		SourceURL:         proto.String("https://fb.me/abc123"),
		SourceApp:         proto.String("facebook"),
		CtwaClid:          proto.String("ARBcdEfGhIjKlMnOpQrStUvWxYz"),
		ShowAdAttribution: proto.Bool(true),
	}
}

func projectOne(t *testing.T, msg *waE2E.Message) MessageEvent {
	t.Helper()

	_, data, publish := ProjectMessage(newMessageEvent(msg), nil)
	if !publish {
		t.Fatal("expected message to be published")
	}

	result, ok := data.(MessageEvent)
	if !ok {
		t.Fatalf("expected MessageEvent, got %T", data)
	}

	return result
}

func TestProjectMessage_AdReferralOnTextMessage(t *testing.T) {
	result := projectOne(t, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("Hola, vi el anuncio"),
			ContextInfo: &waE2E.ContextInfo{
				ExternalAdReply:            ctwaAdReply(),
				EntryPointConversionSource: proto.String("ctwa_ad"),
			},
		},
	})

	if result.AdReferral == nil {
		t.Fatal("expected adReferral to be projected")
	}

	referral := result.AdReferral
	if referral.CtwaClid != "ARBcdEfGhIjKlMnOpQrStUvWxYz" {
		t.Errorf("ctwaClid = %q", referral.CtwaClid)
	}
	if referral.SourceID != "120210000000000000" {
		t.Errorf("sourceId = %q", referral.SourceID)
	}
	if referral.SourceType != "ad" {
		t.Errorf("sourceType = %q", referral.SourceType)
	}
	if referral.SourceURL != "https://fb.me/abc123" {
		t.Errorf("sourceUrl = %q", referral.SourceURL)
	}
	if referral.SourceApp != "facebook" {
		t.Errorf("sourceApp = %q", referral.SourceApp)
	}
	if referral.Title != "Zapatillas 2x1" || referral.Body != "Solo por hoy" {
		t.Errorf("title/body = %q / %q", referral.Title, referral.Body)
	}
	if referral.MediaType != "image" {
		t.Errorf("mediaType = %q", referral.MediaType)
	}
	if referral.ThumbnailURL != "https://scontent.example/thumb.jpg" {
		t.Errorf("thumbnailUrl = %q", referral.ThumbnailURL)
	}
	if referral.ConversionSource != "ctwa_ad" {
		t.Errorf("conversionSource = %q", referral.ConversionSource)
	}
	if !referral.ShowAdAttribution {
		t.Error("expected showAdAttribution to be true")
	}
	if result.Text != "Hola, vi el anuncio" {
		t.Errorf("text = %q", result.Text)
	}
}

// The ad context rides on whatever message type the contact sends first, so the
// media path has to project it too.
func TestProjectMessage_AdReferralOnMediaMessage(t *testing.T) {
	result := projectOne(t, &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Mimetype: proto.String("image/jpeg"),
			Caption:  proto.String("Me interesa esto"),
			ContextInfo: &waE2E.ContextInfo{
				ExternalAdReply:  ctwaAdReply(),
				ConversionSource: proto.String("FB_Ads"),
			},
		},
	})

	if result.AdReferral == nil {
		t.Fatal("expected adReferral to be projected on a media message")
	}
	if result.AdReferral.CtwaClid != "ARBcdEfGhIjKlMnOpQrStUvWxYz" {
		t.Errorf("ctwaClid = %q", result.AdReferral.CtwaClid)
	}
	// Falls back to conversionSource when entryPointConversionSource is absent.
	if result.AdReferral.ConversionSource != "FB_Ads" {
		t.Errorf("conversionSource = %q", result.AdReferral.ConversionSource)
	}
}

// externalAdReply is also used for rich link-preview cards. Without a click ID,
// a source or an attribution flag there is nothing to attribute.
func TestProjectMessage_NoAdReferralForPlainLinkPreview(t *testing.T) {
	result := projectOne(t, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("mira esto https://example.com"),
			ContextInfo: &waE2E.ContextInfo{
				ExternalAdReply: &waE2E.ContextInfo_ExternalAdReplyInfo{
					Title:        proto.String("Example"),
					Body:         proto.String("An example page"),
					ThumbnailURL: proto.String("https://example.com/thumb.jpg"),
					MediaType:    waE2E.ContextInfo_ExternalAdReplyInfo_IMAGE.Enum(),
				},
			},
		},
	})

	if result.AdReferral != nil {
		t.Fatalf("expected no adReferral, got %+v", result.AdReferral)
	}
}

func TestProjectMessage_NoAdReferralWithoutContext(t *testing.T) {
	result := projectOne(t, &waE2E.Message{
		Conversation: proto.String("hola"),
	})

	if result.AdReferral != nil {
		t.Fatalf("expected no adReferral, got %+v", result.AdReferral)
	}
}

// The unified context projection must keep serving the fields it served before.
func TestProjectMessage_ContextInfoStillProjectsReplyAndMentions(t *testing.T) {
	t.Run("extendedText", func(t *testing.T) {
		result := projectOne(t, &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text: proto.String("dale"),
				ContextInfo: &waE2E.ContextInfo{
					StanzaID:     proto.String("3EB0PREV"),
					Participant:  proto.String("5491100000000@s.whatsapp.net"),
					MentionedJID: []string{"5491122222222@s.whatsapp.net"},
					IsForwarded:  proto.Bool(true),
					Expiration:   proto.Uint32(7 * 24 * 60 * 60),
					QuotedMessage: &waE2E.Message{
						Conversation: proto.String("vamos?"),
					},
				},
			},
		})

		if result.ReplyTo == nil {
			t.Fatal("expected replyTo")
		}
		if result.ReplyTo.ID != "3EB0PREV" {
			t.Errorf("replyTo.id = %q", result.ReplyTo.ID)
		}
		if result.ReplyTo.Text != "vamos?" {
			t.Errorf("replyTo.text = %q", result.ReplyTo.Text)
		}
		if result.ReplyTo.Sender == nil || result.ReplyTo.Sender.Phone != "5491100000000" {
			t.Errorf("replyTo.sender = %+v", result.ReplyTo.Sender)
		}
		if !result.ReplyTo.IsForwarded || !result.IsForwarded {
			t.Error("expected forwarded flags")
		}
		if len(result.Mentions) != 1 {
			t.Errorf("mentions = %v", result.Mentions)
		}
		if result.EphemeralExpiration != "7d" {
			t.Errorf("ephemeralExpiration = %q", result.EphemeralExpiration)
		}
	})

	t.Run("media", func(t *testing.T) {
		result := projectOne(t, &waE2E.Message{
			VideoMessage: &waE2E.VideoMessage{
				Mimetype: proto.String("video/mp4"),
				Caption:  proto.String("mira"),
				ContextInfo: &waE2E.ContextInfo{
					StanzaID:     proto.String("3EB0PREV"),
					Participant:  proto.String("5491100000000@s.whatsapp.net"),
					MentionedJID: []string{"5491122222222@s.whatsapp.net"},
					Expiration:   proto.Uint32(24 * 60 * 60),
				},
			},
		})

		if result.ReplyTo == nil || result.ReplyTo.ID != "3EB0PREV" {
			t.Fatalf("replyTo = %+v", result.ReplyTo)
		}
		if len(result.Mentions) != 1 {
			t.Errorf("mentions = %v", result.Mentions)
		}
		// Media messages did not report this before the context projection was unified.
		if result.EphemeralExpiration != "24h" {
			t.Errorf("ephemeralExpiration = %q", result.EphemeralExpiration)
		}
	})
}
