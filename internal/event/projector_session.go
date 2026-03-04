package event

import (
	waEvents "go.mau.fi/whatsmeow/types/events"
)

// ProjectLoggedOut converts a whatsmeow LoggedOut event into a SessionLoggedOutEvent.
func ProjectLoggedOut(evt *waEvents.LoggedOut) (string, any, bool) {
	if evt == nil {
		return TypeLoggedOut, nil, false
	}

	data := SessionLoggedOutEvent{
		Reason: evt.Reason.String(),
	}

	return TypeLoggedOut, data, true
}

// ProjectPairSuccess converts a whatsmeow PairSuccess event into a SessionLoggedInEvent.
func ProjectPairSuccess(evt *waEvents.PairSuccess) (string, any, bool) {
	if evt == nil {
		return TypeLoggedIn, nil, false
	}

	data := SessionLoggedInEvent{
		DeviceID: evt.ID.String(),
	}

	return TypeLoggedIn, data, true
}

// ProjectPairError converts a whatsmeow PairError event into a SessionLoginErrorEvent.
func ProjectPairError(evt *waEvents.PairError) (string, any, bool) {
	if evt == nil {
		return TypeLoginError, nil, false
	}

	data := SessionLoginErrorEvent{
		ID:    evt.ID.String(),
		Error: evt.Error.Error(),
	}

	return TypeLoginError, data, true
}
