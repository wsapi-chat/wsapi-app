package event

import (
	waEvents "go.mau.fi/whatsmeow/types/events"
)

// ProjectNewsletterJoin converts a whatsmeow NewsletterJoin event into a NewsletterEvent.
func ProjectNewsletterJoin(evt *waEvents.NewsletterJoin, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeNewsletter, nil, false
	}

	data := NewsletterEvent{
		ID:     evt.ID.String(),
		Action: "join",
		Name:   evt.ThreadMeta.Name.Text,
	}

	if evt.ViewerMeta != nil {
		data.Role = string(evt.ViewerMeta.Role)
	}

	return TypeNewsletter, data, true
}

// ProjectNewsletterLeave converts a whatsmeow NewsletterLeave event into a NewsletterEvent.
func ProjectNewsletterLeave(evt *waEvents.NewsletterLeave, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeNewsletter, nil, false
	}

	data := NewsletterEvent{
		ID:     evt.ID.String(),
		Action: "leave",
		Role:   string(evt.Role),
	}

	return TypeNewsletter, data, true
}

// ProjectNewsletterMuteChange converts a whatsmeow NewsletterMuteChange event into a NewsletterEvent.
func ProjectNewsletterMuteChange(evt *waEvents.NewsletterMuteChange, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeNewsletter, nil, false
	}

	data := NewsletterEvent{
		ID:     evt.ID.String(),
		Action: "mute",
		Mute:   string(evt.Mute),
	}

	return TypeNewsletter, data, true
}
