package validate

import (
	"testing"

	"github.com/wsapi-chat/wsapi-app/internal/whatsapp"
)

func TestRecipient(t *testing.T) {
	valid := []struct {
		in   string
		note string
	}{
		{"2341234567890", "bare number"},
		{"+2341234567890", "bare number, international prefix"},
		{"2341234567890@s.whatsapp.net", "person"},
		{"120363012345678901@g.us", "group"},
		{"12345-67890@g.us", "legacy group, digits-digits"},
		{"120363012345678901@newsletter", "newsletter"},
		{"12345@lid", "lid — server not on any allowlist, must still pass"},
		{"12345@some.future.server", "unknown server type is WhatsApp's to introduce, not ours to reject"},
		// Rejected when this validator first shipped, on the reasoning that '+' is
		// display notation for a bare number. Not true of what this API accepts:
		// FormatRecipient strips the '+' before it looks for '@', so this form has
		// always been delivered, and rejecting it turned working sends into 400s.
		{"+2341234567890@s.whatsapp.net", "+ prefix on a JID, normalized away by FormatRecipient"},
	}
	for _, c := range valid {
		if !Recipient(c.in) {
			t.Errorf("Recipient(%q) = false, want true (%s)", c.in, c.note)
		}
	}

	invalid := []struct {
		in   string
		note string
	}{
		// The case that motivated this: a template expression leaking into the
		// literal value, each send burning the full 75s ack budget before failing
		// with nothing to point at.
		{"=2341234567890@s.whatsapp.net", "leading = from a template expression"},
		{"=2341234567890", "same, bare"},

		// Other shapes of the same mistake — an unevaluated template reaching us.
		{"{{ $json.phone }}@s.whatsapp.net", "unevaluated template"},
		{"${phone}@s.whatsapp.net", "unevaluated shell/JS template"},
		{"null@s.whatsapp.net", "stringified null"},
		{"undefined", "stringified undefined"},

		{"", "empty"},
		{"@s.whatsapp.net", "no local part"},
		{"2341234567890@", "no server"},
		{"2341234567890@@s.whatsapp.net", "double separator"},
		{"a@b@s.whatsapp.net", "two separators, non-numeric local part"},
		{"234 8100674277@s.whatsapp.net", "embedded space"},
		{"++2341234567890@s.whatsapp.net", "FormatRecipient strips one '+', leaving one in the local part"},
		{"+@s.whatsapp.net", "nothing but the prefix"},
		{"2341234567890:3@s.whatsapp.net", "device JID — whatsmeow rejects these as recipients"},
		{"01234567890", "national trunk prefix, no country code"},
		{"0", "single zero"},
	}
	for _, c := range invalid {
		if Recipient(c.in) {
			t.Errorf("Recipient(%q) = true, want false (%s)", c.in, c.note)
		}
	}
}

// The '+' is the one piece of notation the sender normalizes away, so it is the
// one place the validator and the sender can disagree without either looking
// wrong alone. Asserting both halves means a change to either side breaks this
// test rather than real sends.
func TestPlusPrefixAgreesWithTheSender(t *testing.T) {
	for _, in := range []string{"+2341234567890@s.whatsapp.net", "+2341234567890"} {
		if !Recipient(in) {
			t.Errorf("Recipient(%q) = false, want true", in)
		}
		if jid := whatsapp.FormatRecipient(in); !jidUserRegex.MatchString(jid.User) {
			t.Errorf("FormatRecipient(%q) = %q, whose local part is not routable — accepting it above is wrong", in, jid)
		}
	}

	// One '+' and no more: whatever the sender leaves in the local part stays rejected.
	const doubled = "++2341234567890@s.whatsapp.net"
	if Recipient(doubled) {
		t.Errorf("Recipient(%q) = true, want false", doubled)
	}
	if jid := whatsapp.FormatRecipient(doubled); jidUserRegex.MatchString(jid.User) {
		t.Errorf("FormatRecipient(%q) = %q, now routable — the validator should follow", doubled, jid)
	}
}

// The rule is only worth anything if it is actually wired to the struct tag —
// Phone() existed for months, documented with this exact rationale, and was
// never registered as a validator, so nothing ever called it.
func TestRecipientTagIsRegistered(t *testing.T) {
	type sendRequest struct {
		To string `json:"to" validate:"required,recipient"`
	}

	if err := Struct(&sendRequest{To: "2341234567890@s.whatsapp.net"}); err != nil {
		t.Errorf("Struct(valid recipient) = %v, want nil", err)
	}
	if err := Struct(&sendRequest{To: "=2341234567890@s.whatsapp.net"}); err == nil {
		t.Error("Struct(malformed recipient) = nil, want a validation error — the tag is not registered")
	}
}

// Every shape seen on real chat JIDs must pass. If this fails, the validator is
// stricter than reality and will reject valid sends.
func TestRecipientAcceptsRealWorldShapes(t *testing.T) {
	for _, server := range []string{"s.whatsapp.net", "g.us", "newsletter", "bot", "lid"} {
		for _, user := range []string{"2341234567890", "120363012345678901", "12345-67890"} {
			jid := user + "@" + server
			if !Recipient(jid) {
				t.Errorf("Recipient(%q) = false, want true (seen in real traffic)", jid)
			}
		}
	}
}
