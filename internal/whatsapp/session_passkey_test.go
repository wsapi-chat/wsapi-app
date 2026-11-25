package whatsapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	waTypes "go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
)

// assertion builds a credential.toJSON()-shaped body around a clientDataJSON.
func assertion(clientDataJSON string) []byte {
	cd := base64.RawURLEncoding.EncodeToString([]byte(clientDataJSON))
	return []byte(`{"id":"AQ","rawId":"AQ","type":"public-key","response":{"clientDataJSON":"` + cd +
		`","authenticatorData":"AQ","signature":"AQ","userHandle":null}}`)
}

func newPasskeyTestSession() *SessionService {
	return &SessionService{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func passkeyRequest() *waEvents.PairPasskeyRequest {
	return &waEvents.PairPasskeyRequest{PublicKey: &waTypes.WebAuthnPublicKey{
		Challenge:        []byte{1, 2, 3, 250},
		Timeout:          60000,
		RelyingPartID:    "web.whatsapp.com",
		UserVerification: "required",
		AllowCredentials: []waTypes.AllowedCredential{{ID: []byte{9, 9}, Type: "public-key", Transports: []string{"hybrid", "internal"}}},
	}}
}

func TestPasskeyStateDefaultsToNone(t *testing.T) {
	s := newPasskeyTestSession()
	if got := s.PasskeyState(); got.Status != PasskeyStatusNone || got.Challenge != nil {
		t.Fatalf("unexpected initial state: %+v", got)
	}
	if s.passkeyPending() {
		t.Fatal("nothing should be pending initially")
	}
}

func TestPasskeyRequestExposesChallengeAndBlocksNewCodes(t *testing.T) {
	s := newPasskeyTestSession()
	s.handlePasskeyEvent(passkeyRequest())

	st := s.PasskeyState()
	if st.Status != PasskeyStatusRequired {
		t.Fatalf("status = %q, want required", st.Status)
	}
	if st.Origin != PasskeyOrigin {
		t.Fatalf("origin = %q", st.Origin)
	}
	var opts map[string]any
	if err := json.Unmarshal(st.Challenge, &opts); err != nil {
		t.Fatalf("challenge is not JSON: %v", err)
	}
	// Binary fields must be base64url without padding, the format
	// PublicKeyCredential.parseRequestOptionsFromJSON expects.
	if opts["challenge"] != "AQID-g" {
		t.Fatalf("challenge = %v, want base64url AQID-g", opts["challenge"])
	}
	if opts["rpId"] != "web.whatsapp.com" {
		t.Fatalf("rpId = %v", opts["rpId"])
	}
	if !s.passkeyPending() {
		t.Fatal("a required passkey step must block QR / pair-code generation")
	}
}

func TestPasskeyConfirmationCodeFlow(t *testing.T) {
	s := newPasskeyTestSession()
	s.handlePasskeyEvent(passkeyRequest())
	s.handlePasskeyEvent(&waEvents.PairPasskeyConfirmation{Code: "ABCD-EFGH", SkipHandoffUX: false})

	st := s.PasskeyState()
	if st.Status != PasskeyStatusConfirmationRequired || st.ConfirmationCode != "ABCD-EFGH" {
		t.Fatalf("unexpected state: %+v", st)
	}
	if st.Challenge != nil {
		t.Fatal("challenge should not be exposed once it has been answered")
	}
	if !s.passkeyPending() {
		t.Fatal("confirmation step must still block new codes")
	}
}

// The confirmation goes out by itself (the user approves on the phone), and a
// later call to ConfirmPasskey is a harmless no-op.
func TestPasskeyConfirmationIsSentAutomatically(t *testing.T) {
	s := newPasskeyTestSession()
	s.handlePasskeyEvent(passkeyRequest())
	s.handlePasskeyEvent(&waEvents.PairPasskeyConfirmation{Code: "ABCD-EFGH"})
	// No client in tests: mark it sent the way sendCodeConfirmation would.
	s.passkey.mu.Lock()
	s.passkey.confirmationSent = true
	s.passkey.mu.Unlock()

	st := s.PasskeyState()
	if st.Status != PasskeyStatusConfirmationRequired || !st.ConfirmationSent || st.ConfirmationCode == "" {
		t.Fatalf("state = %+v, want confirmation_required with the code, sent", st)
	}
	if err := s.ConfirmPasskey(context.Background()); err != nil {
		t.Fatalf("ConfirmPasskey after automatic send = %v, want nil", err)
	}
}

func TestPasskeyErrorUnblocks(t *testing.T) {
	s := newPasskeyTestSession()
	s.handlePasskeyEvent(passkeyRequest())
	s.handlePasskeyEvent(&waEvents.PairPasskeyError{Error: errors.New("boom")})

	st := s.PasskeyState()
	if st.Status != PasskeyStatusError || st.Error != "boom" {
		t.Fatalf("unexpected state: %+v", st)
	}
	if s.passkeyPending() {
		t.Fatal("a failed passkey step must not block a new attempt")
	}
}

func TestPasskeyPairSuccessResets(t *testing.T) {
	s := newPasskeyTestSession()
	s.handlePasskeyEvent(passkeyRequest())
	s.handlePasskeyEvent(&waEvents.PairSuccess{})
	if got := s.PasskeyState(); got.Status != PasskeyStatusNone {
		t.Fatalf("status after pair success = %q", got.Status)
	}
}

func TestPasskeyExpires(t *testing.T) {
	s := newPasskeyTestSession()
	s.handlePasskeyEvent(passkeyRequest())
	s.passkey.mu.Lock()
	s.passkey.requestedAt = time.Now().Add(-passkeyPendingLifetime - time.Second)
	s.passkey.mu.Unlock()

	if s.passkeyPending() {
		t.Fatal("expired step must not block")
	}
	if st := s.PasskeyState(); st.Status != PasskeyStatusError || st.Challenge != nil {
		t.Fatalf("expired state: %+v", st)
	}
}

func TestSubmitPasskeyValidation(t *testing.T) {
	s := newPasskeyTestSession()
	ctx := context.Background()

	if _, err := s.SubmitPasskeyResponse(ctx, []byte("not json")); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if _, err := s.SubmitPasskeyResponse(ctx, []byte(`{"id":"x"}`)); err == nil {
		t.Fatal("expected error for incomplete assertion")
	}

	valid := assertion(`{"type":"webauthn.get","challenge":"AQID-g","origin":"https://web.whatsapp.com"}`)
	if _, err := s.SubmitPasskeyResponse(ctx, valid); !errors.Is(err, ErrNoPasskeyPending) {
		t.Fatalf("err = %v, want ErrNoPasskeyPending with nothing pending", err)
	}
	if err := s.ConfirmPasskey(ctx); !errors.Is(err, ErrNoPasskeyPending) {
		t.Fatalf("confirm err = %v, want ErrNoPasskeyPending", err)
	}
}

// Mistakes that WhatsApp would reject opaquely are caught before sending, and
// they don't consume the pending step.
func TestSubmitPasskeyRejectsWrongOriginTypeOrChallenge(t *testing.T) {
	s := newPasskeyTestSession()
	s.handlePasskeyEvent(passkeyRequest())
	ctx := context.Background()

	cases := map[string]string{
		"origin":    `{"type":"webauthn.get","challenge":"AQID-g","origin":"https://wsapi.chat"}`,
		"type":      `{"type":"webauthn.create","challenge":"AQID-g","origin":"https://web.whatsapp.com"}`,
		"challenge": `{"type":"webauthn.get","challenge":"BBBB","origin":"https://web.whatsapp.com"}`,
	}
	for name, cd := range cases {
		if _, err := s.SubmitPasskeyResponse(ctx, assertion(cd)); err == nil || errors.Is(err, ErrNoPasskeyPending) {
			t.Fatalf("%s: err = %v, want a validation error", name, err)
		}
		if st := s.PasskeyState(); st.Status != PasskeyStatusRequired {
			t.Fatalf("%s: status = %q, a local rejection must leave the step pending", name, st.Status)
		}
	}
}

func TestPasskeySourceIsReported(t *testing.T) {
	s := newPasskeyTestSession()
	s.handlePasskeyEvent(passkeyRequest())
	if st := s.PasskeyState(); st.Source != PasskeySourceNotification {
		t.Fatalf("source = %q, want notification", st.Source)
	}
	s.setPasskeyRequired(passkeyRequest().PublicKey, PasskeySourceRequested)
	if st := s.PasskeyState(); st.Source != PasskeySourceRequested || st.Status != PasskeyStatusRequired {
		t.Fatalf("state = %+v, want required/requested", st)
	}
}

func TestPasskeyDisconnectFailsActiveStep(t *testing.T) {
	s := newPasskeyTestSession()
	s.handlePasskeyEvent(&waEvents.Disconnected{})
	if st := s.PasskeyState(); st.Status != PasskeyStatusNone {
		t.Fatalf("disconnect with nothing pending: status = %q, want none", st.Status)
	}
	s.handlePasskeyEvent(passkeyRequest())
	s.handlePasskeyEvent(&waEvents.Disconnected{})
	st := s.PasskeyState()
	if st.Status != PasskeyStatusError || st.Error == "" {
		t.Fatalf("state = %+v, want error after the connection dropped", st)
	}
	if s.passkeyPending() {
		t.Fatal("a dropped connection must unblock new QR / pair codes")
	}
}
