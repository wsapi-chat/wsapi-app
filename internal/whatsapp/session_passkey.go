package whatsapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	waTypes "go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
)

// Passkey pairing ("Shortcake").
//
// On some accounts WhatsApp no longer finishes a link after the QR scan or the
// pair code. It sends a passkey_prologue_request instead, carrying standard
// WebAuthn request options, and waits for an assertion from the account
// owner's passkey. whatsmeow implements the protocol and surfaces it as
// PairPasskeyRequest / PairPasskeyConfirmation / PairPasskeyError; this file
// keeps that state per instance so the API can expose it:
//
//	GET  /session/passkey          → current state, incl. the WebAuthn options
//	POST /session/passkey/request  → ask WhatsApp for a challenge without waiting
//	                                 for its notification (fallback, see RequestPasskey)
//	POST /session/passkey          → submit the assertion (credential.toJSON())
//	POST /session/passkey/confirm  → confirm the code matches the phone
//
// The assertion can only be produced by a browser on the https://web.whatsapp.com
// origin (WebAuthn binds the origin into clientDataJSON), so a server cannot do
// it alone — a browser extension or a console snippet has to run it there.
//
// The whole exchange lives in the whatsmeow client's memory: the handoff key
// derived from the scanned QR, the ephemeral keys, and the websocket itself.
// Anything that disconnects the client in the meantime kills it, which is why
// the QR and pair-code generators refuse to run while a passkey step is pending
// (see ErrPasskeyPending) instead of disconnecting as they normally do.

// Passkey pairing states reported by GET /session/passkey.
const (
	PasskeyStatusNone                 = "none"
	PasskeyStatusRequired             = "required"
	PasskeyStatusSubmitted            = "submitted"
	PasskeyStatusConfirmationRequired = "confirmation_required"
	PasskeyStatusError                = "error"
)

// passkeyPendingLifetime bounds how long a passkey step blocks QR/pair-code
// generation. whatsmeow only honours the QR handoff proof for 5 minutes after
// the request, and the WebAuthn challenge itself carries a shorter timeout, so
// past this point the attempt is dead and a fresh QR is the only way forward.
const passkeyPendingLifetime = 5 * time.Minute

// passkeyAutoConfirmDelay is how long the session waits before sending the
// confirmation itself when WhatsApp says the code doesn't need showing. A live
// QR channel sends it on its own; the delay lets that win, so the request
// isn't sent twice. Pair-code pairings have no QR channel, so for those this is
// the only sender.
const passkeyAutoConfirmDelay = 3 * time.Second

// passkeyConnectSettle is how long RequestPasskey waits after opening a fresh
// connection: WhatsApp sends pair-device right after the handshake, and the QR
// handlers it wakes up have to be running before they can be stopped.
const passkeyConnectSettle = 3 * time.Second

// passkeyReuseWindow: RequestPasskey hands back a challenge it requested this
// recently (a double click) instead of starting over. Older ones are replaced:
// the connection they live on is closed by WhatsApp ~3.5 minutes after it opens,
// so a retry should get a fresh one.
const passkeyReuseWindow = 30 * time.Second

// ErrPasskeyPending is returned by QR / pair-code generation while WhatsApp is
// waiting for a passkey assertion. Generating a new code would disconnect the
// client and discard the in-progress pairing.
var ErrPasskeyPending = errors.New("passkey verification pending: complete it via /session/passkey or wait for it to expire")

// ErrPasskeyNotConnected is returned by RequestPasskey when the connection to
// WhatsApp it opened didn't come up.
var ErrPasskeyNotConnected = errors.New("could not connect to whatsapp to request the passkey challenge; try again")

// ErrNoPasskeyPending is returned when a passkey action is called with no
// matching pending step.
var ErrNoPasskeyPending = errors.New("no passkey verification pending")

// Where a passkey challenge came from.
const (
	// PasskeySourceNotification: WhatsApp pushed it after the QR scan / pair
	// code. whatsmeow derives a handoff proof from the QR, so the phone is
	// usually not asked to approve again.
	PasskeySourceNotification = "notification"
	// PasskeySourceRequested: fetched on demand via POST /session/passkey/request.
	// There is no handoff proof, so WhatsApp asks the owner to approve the new
	// device on the phone (status confirmation_required).
	PasskeySourceRequested = "requested"
)

// PasskeyState is the public view of an instance's passkey pairing step.
type PasskeyState struct {
	Status string `json:"status"`
	// Source is "notification" or "requested" (see PasskeySource*).
	Source string `json:"source,omitempty"`
	// Challenge holds the WebAuthn PublicKeyCredentialRequestOptions sent by
	// WhatsApp, binary fields base64url-encoded (the JSON form accepted by
	// PublicKeyCredential.parseRequestOptionsFromJSON). Pass it to
	// navigator.credentials.get on https://web.whatsapp.com.
	Challenge json.RawMessage `json:"challenge,omitempty"`
	// Origin is where the challenge must be signed.
	Origin string `json:"origin,omitempty"`
	// ConfirmationCode is set in confirmation_required: show it to the user and
	// have them check it matches the one on their phone before confirming.
	ConfirmationCode string `json:"confirmationCode,omitempty"`
	// ConfirmationSent is true once the confirmation went out (it is sent
	// automatically): the link now waits for the user to approve on the phone.
	ConfirmationSent bool       `json:"confirmationSent,omitempty"`
	Error            string     `json:"error,omitempty"`
	RequestedAt      *time.Time `json:"requestedAt,omitempty"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
}

// PasskeyOrigin is the origin WhatsApp expects the assertion to come from.
const PasskeyOrigin = "https://web.whatsapp.com"

type passkeyTracker struct {
	mu               sync.Mutex
	status           string
	challenge        json.RawMessage
	challengeValue   string // base64url WebAuthn challenge, to check the assertion against
	source           string
	confirmationCode string
	confirmationSent bool
	lastError        string
	requestedAt      time.Time
	// attempt increments on every new request so a delayed auto-confirm from
	// an earlier attempt can tell it is stale.
	attempt uint64
}

// activeLocked reports whether a passkey step is in progress and still within
// its lifetime. Caller holds mu.
func (t *passkeyTracker) activeLocked(now time.Time) bool {
	switch t.status {
	case PasskeyStatusRequired, PasskeyStatusSubmitted, PasskeyStatusConfirmationRequired:
		return now.Sub(t.requestedAt) < passkeyPendingLifetime
	}
	return false
}

func (t *passkeyTracker) resetLocked() {
	t.status = PasskeyStatusNone
	t.challenge = nil
	t.challengeValue = ""
	t.source = ""
	t.confirmationCode = ""
	t.confirmationSent = false
	t.lastError = ""
	t.requestedAt = time.Time{}
}

// handlePasskeyEvent is registered as a whatsmeow event handler. It runs inside
// dispatchEvent, so it must not block: state updates only, slow work goes to a
// goroutine.
func (s *SessionService) handlePasskeyEvent(rawEvt any) {
	t := &s.passkey
	switch evt := rawEvt.(type) {
	case *waEvents.PairPasskeyRequest:
		s.setPasskeyRequired(evt.PublicKey, PasskeySourceNotification)
		// Inside dispatchEvent: dispatching again here could deadlock.
		go s.stopQRRotation("passkey requested by whatsapp")

	case *waEvents.PairPasskeyConfirmation:
		t.mu.Lock()
		attempt := t.attempt
		if evt.SkipHandoffUX {
			t.status = PasskeyStatusSubmitted
		} else {
			t.status = PasskeyStatusConfirmationRequired
			t.confirmationCode = evt.Code
		}
		t.mu.Unlock()
		s.logger.Info("passkey accepted by whatsapp",
			"needsCodeConfirmation", !evt.SkipHandoffUX, "hasCode", evt.Code != "")
		if evt.SkipHandoffUX {
			go s.autoConfirmPasskey(attempt)
		} else {
			// The code is for the user to compare on the phone, which is where
			// they approve. The confirmation itself carries no user decision,
			// so send it now rather than waiting for a click on the page.
			go s.sendCodeConfirmation(attempt)
		}

	case *waEvents.PairPasskeyError:
		t.mu.Lock()
		t.status = PasskeyStatusError
		t.lastError = evt.Error.Error()
		t.mu.Unlock()
		s.logger.Warn("passkey pairing failed", "error", evt.Error, "continuation", evt.Continuation)

	case *waEvents.Disconnected:
		// WhatsApp closes an unpaired connection about 3.5 minutes after it
		// opens, and the passkey exchange lives on that connection. Expected
		// disconnects (ours, or the reconnect after PairSuccess) emit nothing.
		t.mu.Lock()
		active := t.activeLocked(time.Now())
		var age time.Duration
		if active {
			age = time.Since(t.requestedAt)
			t.status = PasskeyStatusError
			t.lastError = "whatsapp closed the connection before the passkey step finished; start again"
		}
		t.mu.Unlock()
		if active {
			s.logger.Warn("connection closed during passkey step", "sinceRequestMs", age.Milliseconds())
		}

	case *waEvents.PairSuccess, *waEvents.PairError, *waEvents.LoggedOut:
		t.mu.Lock()
		wasActive := t.status != PasskeyStatusNone
		t.resetLocked()
		t.mu.Unlock()
		if wasActive {
			s.logger.Info("passkey pairing step finished", "event", fmt.Sprintf("%T", rawEvt))
		}
	}
}

// setPasskeyRequired records a new challenge, from WhatsApp's notification or
// from RequestPasskey.
func (s *SessionService) setPasskeyRequired(pk *waTypes.WebAuthnPublicKey, source string) {
	challenge, err := json.Marshal(pk)
	t := &s.passkey
	t.mu.Lock()
	t.attempt++
	t.resetLocked()
	t.requestedAt = time.Now()
	t.source = source
	if err != nil || pk == nil {
		t.status = PasskeyStatusError
		t.lastError = fmt.Sprintf("failed to encode passkey challenge: %v", err)
	} else {
		t.status = PasskeyStatusRequired
		t.challenge = challenge
		t.challengeValue = base64.RawURLEncoding.EncodeToString(pk.Challenge)
	}
	t.mu.Unlock()

	if pk == nil {
		s.logger.Warn("whatsapp requested passkey verification without options", "source", source)
		return
	}
	transports := []string{}
	for _, c := range pk.AllowCredentials {
		transports = append(transports, c.Transports...)
	}
	s.logger.Info("whatsapp requested passkey verification to finish pairing",
		"source", source,
		"rpId", pk.RelyingPartID,
		"timeoutMs", pk.Timeout,
		"userVerification", pk.UserVerification,
		"allowCredentials", len(pk.AllowCredentials),
		"transports", transports,
		"extensions", len(pk.Extensions),
	)
}

// stopQRRotation retires every whatsmeow QR channel on this client.
//
// A QR channel's emitter disconnects the client once it runs out of codes
// (~160s after the connection's pair-device), and channels from earlier QR polls
// stay registered and start a new emitter on every reconnect. During a passkey
// step that disconnect kills the pairing, so the channels have to go.
//
// whatsmeow has no API for it; a QR channel closes itself — stopping its
// emitter without disconnecting — on any terminal event. ClientOutdated is one
// that nothing else in this service handles (the projector discards it), so it
// is dispatched as that signal. Anyone adding a ClientOutdated handler must
// account for this synthetic one.
func (s *SessionService) stopQRRotation(reason string) {
	if s.client == nil {
		return
	}
	s.client.DangerousInternals().DispatchEvent(&waEvents.ClientOutdated{}) //nolint:staticcheck // no public API to stop a QR channel
	s.logger.Info("stopped QR code rotation for the passkey step", "reason", reason)
}

// RequestPasskey asks WhatsApp for a passkey challenge instead of waiting for it
// to push one after a QR scan.
//
// The challenge is for a discoverable credential (no allowCredentials): the
// passkey itself tells WhatsApp which account is linking, so no QR or pair code
// is needed. If there is no connection, one is opened (without a QR channel).
// It also covers accounts where WhatsApp asks for the passkey on the phone but
// the notification never reaches the companion.
//
// Without the QR handoff proof WhatsApp asks the owner to approve the new device
// on the phone, so expect confirmation_required after the assertion.
func (s *SessionService) RequestPasskey(ctx context.Context) (PasskeyState, error) {
	if s.client.Store.ID != nil {
		return PasskeyState{}, fmt.Errorf("device already registered")
	}
	t := &s.passkey
	t.mu.Lock()
	now := time.Now()
	active := t.activeLocked(now)
	reuse := active && t.status == PasskeyStatusRequired && t.source == PasskeySourceRequested &&
		now.Sub(t.requestedAt) < passkeyReuseWindow
	t.mu.Unlock()
	if reuse {
		return s.PasskeyState(), nil
	}
	if active {
		// Start over on a fresh connection; the old challenge dies with it.
		s.logger.Info("replacing pending passkey step with a fresh connection")
		s.client.Disconnect()
		s.clearPasskey()
	}
	if !s.client.IsConnected() {
		s.logger.Info("connecting to request a passkey challenge")
		if err := s.client.Connect(); err != nil {
			return PasskeyState{}, fmt.Errorf("%w: connecting to whatsapp: %v", ErrUpstream, err)
		}
		select {
		case <-time.After(passkeyConnectSettle):
		case <-ctx.Done():
			return PasskeyState{}, ctx.Err()
		}
		if !s.client.IsConnected() {
			return PasskeyState{}, ErrPasskeyNotConnected
		}
	}
	s.stopQRRotation("passkey challenge requested")

	pk, err := s.client.DangerousInternals().GetPasskeyRequestOptions(ctx) //nolint:staticcheck // only exposed via internals
	if err != nil {
		s.logger.Warn("whatsapp refused passkey challenge request", "error", err)
		return PasskeyState{}, fmt.Errorf("%w: whatsapp did not return a passkey challenge (the account may not use a passkey): %v", ErrUpstream, err)
	}
	s.setPasskeyRequired(pk, PasskeySourceRequested)
	return s.PasskeyState(), nil
}

// clientData is the subset of WebAuthn's clientDataJSON checked before the
// assertion is sent to WhatsApp. None of it is secret.
type clientData struct {
	Type        string `json:"type"`
	Challenge   string `json:"challenge"`
	Origin      string `json:"origin"`
	CrossOrigin bool   `json:"crossOrigin"`
}

// sendCodeConfirmation sends the confirmation for a step WhatsApp answered
// with a code (no QR handoff proof). The status stays confirmation_required,
// with the code, until PairSuccess, so the page can keep showing it while the
// user approves on the phone.
func (s *SessionService) sendCodeConfirmation(attempt uint64) {
	t := &s.passkey
	t.mu.Lock()
	stale := t.attempt != attempt || t.status != PasskeyStatusConfirmationRequired || t.confirmationSent
	if !stale {
		t.confirmationSent = true
	}
	t.mu.Unlock()
	if stale || s.client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.client.SendPasskeyConfirmation(ctx); err != nil {
		t.mu.Lock()
		if t.attempt == attempt {
			t.status = PasskeyStatusError
			t.lastError = err.Error()
		}
		t.mu.Unlock()
		s.logger.Warn("passkey confirmation failed", "error", err)
		return
	}
	s.logger.Info("passkey confirmation sent to whatsapp; waiting for approval on the phone")
}

func (s *SessionService) autoConfirmPasskey(attempt uint64) {
	time.Sleep(passkeyAutoConfirmDelay)

	t := &s.passkey
	t.mu.Lock()
	stale := t.attempt != attempt || t.status != PasskeyStatusSubmitted
	t.mu.Unlock()
	if stale || s.client == nil || s.client.Store.ID != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.client.SendPasskeyConfirmation(ctx); err != nil {
		// A QR channel normally gets there first and clears the linking cache;
		// that's the expected "error" here and not worth surfacing.
		s.logger.Debug("automatic passkey confirmation not sent", "error", err)
	}
}

// PasskeyState returns the current passkey pairing state.
func (s *SessionService) PasskeyState() PasskeyState {
	t := &s.passkey
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	if t.status == "" {
		t.status = PasskeyStatusNone
	}
	if t.status != PasskeyStatusNone && t.status != PasskeyStatusError && !t.activeLocked(now) {
		t.status = PasskeyStatusError
		t.lastError = "passkey verification expired; generate a new QR or pair code"
	}

	st := PasskeyState{Status: t.status, Source: t.source, Error: t.lastError}
	if st.Status == "" {
		st.Status = PasskeyStatusNone
	}
	if !t.requestedAt.IsZero() {
		req := t.requestedAt
		exp := req.Add(passkeyPendingLifetime)
		st.RequestedAt, st.ExpiresAt = &req, &exp
	}
	if t.status == PasskeyStatusRequired {
		st.Challenge = t.challenge
		st.Origin = PasskeyOrigin
	}
	if t.status == PasskeyStatusConfirmationRequired {
		st.ConfirmationCode = t.confirmationCode
		st.ConfirmationSent = t.confirmationSent
	}
	return st
}

// passkeyPending reports whether a passkey step is blocking new pairing codes.
func (s *SessionService) passkeyPending() bool {
	t := &s.passkey
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.activeLocked(time.Now())
}

// clearPasskey drops any passkey state. Called when a new pairing attempt
// starts so an expired or failed step doesn't linger in the status.
func (s *SessionService) clearPasskey() {
	t := &s.passkey
	t.mu.Lock()
	t.resetLocked()
	t.mu.Unlock()
}

// SubmitPasskeyResponse forwards a WebAuthn assertion to WhatsApp. raw is the
// JSON produced by PublicKeyCredential.toJSON() in the browser.
func (s *SessionService) SubmitPasskeyResponse(ctx context.Context, raw []byte) (PasskeyState, error) {
	var resp waTypes.WebAuthnResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return PasskeyState{}, fmt.Errorf("invalid WebAuthn response: %v", err)
	}
	if len(resp.RawID) == 0 || len(resp.Response.Signature) == 0 || len(resp.Response.ClientDataJSON) == 0 {
		return PasskeyState{}, errors.New("WebAuthn response is missing rawId, signature or clientDataJSON")
	}

	// Catch the usual mistakes here, with a clear message, instead of letting
	// WhatsApp reject the assertion opaquely and burn the attempt.
	var cd clientData
	if err := json.Unmarshal(resp.Response.ClientDataJSON, &cd); err != nil {
		return PasskeyState{}, fmt.Errorf("clientDataJSON is not valid JSON: %v", err)
	}
	userHandle := resp.Response.UserHandle != nil && len(*resp.Response.UserHandle) > 0

	t := &s.passkey
	t.mu.Lock()
	if t.status != PasskeyStatusRequired || !t.activeLocked(time.Now()) {
		status := t.status
		t.mu.Unlock()
		s.logger.Warn("passkey response received with no challenge pending", "status", status)
		return PasskeyState{}, ErrNoPasskeyPending
	}
	expectedChallenge := t.challengeValue
	attempt := t.attempt
	source := t.source
	t.mu.Unlock()

	logAttrs := []any{
		"source", source,
		"clientDataType", cd.Type,
		"origin", cd.Origin,
		"crossOrigin", cd.CrossOrigin,
		"challengeMatches", cd.Challenge == expectedChallenge,
		"authenticatorDataBytes", len(resp.Response.AuthenticatorData),
		"signatureBytes", len(resp.Response.Signature),
		"hasUserHandle", userHandle,
	}
	switch {
	case cd.Type != "webauthn.get":
		s.logger.Warn("passkey response rejected locally", append(logAttrs, "reason", "wrong type")...)
		return PasskeyState{}, fmt.Errorf("clientDataJSON type is %q, expected webauthn.get", cd.Type)
	case cd.Origin != PasskeyOrigin:
		s.logger.Warn("passkey response rejected locally", append(logAttrs, "reason", "wrong origin")...)
		return PasskeyState{}, fmt.Errorf("the passkey was used on %q; it must be used on %s", cd.Origin, PasskeyOrigin)
	case expectedChallenge != "" && cd.Challenge != expectedChallenge:
		s.logger.Warn("passkey response rejected locally", append(logAttrs, "reason", "stale challenge")...)
		return PasskeyState{}, errors.New("the passkey signed a different challenge (an older attempt?); sign the current one")
	}

	t.mu.Lock()
	if t.attempt != attempt || t.status != PasskeyStatusRequired {
		t.mu.Unlock()
		return PasskeyState{}, ErrNoPasskeyPending
	}
	t.status = PasskeyStatusSubmitted
	t.mu.Unlock()

	s.logger.Info("sending passkey response to whatsapp", logAttrs...)
	start := time.Now()
	if err := s.client.SendPasskeyResponse(ctx, &resp); err != nil {
		t.mu.Lock()
		t.status = PasskeyStatusError
		t.lastError = err.Error()
		t.mu.Unlock()
		s.logger.Warn("whatsapp rejected passkey response", "error", err, "elapsedMs", time.Since(start).Milliseconds())
		return PasskeyState{}, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	s.logger.Info("passkey response accepted by whatsapp", "elapsedMs", time.Since(start).Milliseconds())
	return s.PasskeyState(), nil
}

// ConfirmPasskey sends the confirmation for a step in confirmation_required.
// The session sends it on its own as soon as WhatsApp returns the code, so this
// is only a fallback and a no-op once it went out. Kept for API compatibility.
func (s *SessionService) ConfirmPasskey(ctx context.Context) error {
	t := &s.passkey
	t.mu.Lock()
	if t.status != PasskeyStatusConfirmationRequired || !t.activeLocked(time.Now()) {
		t.mu.Unlock()
		return ErrNoPasskeyPending
	}
	if t.confirmationSent {
		t.mu.Unlock()
		return nil
	}
	t.confirmationSent = true
	t.mu.Unlock()

	if err := s.client.SendPasskeyConfirmation(ctx); err != nil {
		t.mu.Lock()
		t.status = PasskeyStatusError
		t.lastError = err.Error()
		t.mu.Unlock()
		s.logger.Warn("passkey confirmation failed", "error", err)
		return fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	s.logger.Info("passkey confirmation sent to whatsapp")
	return nil
}
