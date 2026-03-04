package event

import (
	"github.com/wsapi-chat/wsapi-app/internal/identity"
	waTypes "go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
)

// ProjectGroupInfo converts a whatsmeow GroupInfo event into a GroupEvent or
// ChatSettingEvent (when the change is ephemeral-related).
func ProjectGroupInfo(evt *waEvents.GroupInfo, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeGroup, nil, false
	}

	chatID := projectChatID(evt.JID, pctx)
	if chatID == "" {
		return TypeGroup, nil, false
	}

	// Ephemeral settings change returns a different event type.
	if evt.Ephemeral != nil {
		return projectGroupEphemeral(evt, pctx)
	}

	// Resolve sender JID: prefer Sender, fall back to SenderPN.
	senderJID := resolveSenderJID(evt.Sender, evt.SenderPN)
	// Use both Sender and SenderPN as primary/alt for identity resolution.
	senderAlt := resolveSenderJID(evt.SenderPN, evt.Sender)

	data := GroupEvent{
		ID:     chatID,
		Sender: resolveSender(senderJID, false, senderAlt, pctx),
	}

	if !evt.Timestamp.IsZero() {
		data.Timestamp = evt.Timestamp
	}

	// Topic/description change
	if topic := evt.Topic; topic != nil {
		data.Description = &GroupDescriptionInfo{
			Topic: topic.Topic,
		}
		if evt.Timestamp.IsZero() {
			data.Timestamp = topic.TopicSetAt
		}
	}

	// Name change
	if name := evt.Name; name != nil {
		data.Name = &GroupNameInfo{
			Name: name.Name,
		}
	}

	// Locked status change
	if locked := evt.Locked; locked != nil {
		data.Locked = &GroupLockedInfo{
			IsLocked: locked.IsLocked,
		}
	}

	// Announce status change
	if announce := evt.Announce; announce != nil {
		data.Announce = &GroupAnnounceInfo{
			IsAnnounce: announce.IsAnnounce,
		}
	}

	// Members leaving
	if evt.Leave != nil {
		data.Leave = resolveMembers(evt.Leave, pctx)
	}

	// Members joining
	if evt.Join != nil {
		data.Join = resolveMembers(evt.Join, pctx)
	}

	// Members promoted to admin
	if evt.Promote != nil {
		data.Promote = resolveMembers(evt.Promote, pctx)
	}

	// Members demoted from admin
	if evt.Demote != nil {
		data.Demote = resolveMembers(evt.Demote, pctx)
	}

	// Group linked to community
	if link := evt.Link; link != nil {
		data.Link = &GroupLinkInfo{
			Type:                string(link.Type),
			GroupID:             link.Group.JID.String(),
			GroupName:           link.Group.Name,
			IsAnnouncementGroup: link.Group.IsDefaultSubGroup,
		}
	}

	// Group unlinked from community
	if unlink := evt.Unlink; unlink != nil {
		data.Unlink = &GroupUnlinkInfo{
			Type:    string(unlink.Type),
			Reason:  string(unlink.UnlinkReason),
			GroupID: unlink.Group.JID.String(),
		}
	}

	// Membership approval mode change
	if approval := evt.MembershipApprovalMode; approval != nil {
		data.MembershipApproval = &GroupMembershipApprovalInfo{
			IsRequired: approval.IsJoinApprovalRequired,
		}
	}

	// Group deleted
	if del := evt.Delete; del != nil {
		data.Delete = &GroupDeleteInfo{
			Reason: del.DeleteReason,
		}
	}

	// Group suspension
	if evt.Suspended {
		suspended := true
		data.Suspended = &suspended
	}
	if evt.Unsuspended {
		suspended := false
		data.Suspended = &suspended
	}

	// Join reason
	if evt.JoinReason != "" {
		data.JoinReason = evt.JoinReason
	}

	return TypeGroup, data, true
}

// ProjectJoinedGroup converts a whatsmeow JoinedGroup event into a GroupEvent.
func ProjectJoinedGroup(evt *waEvents.JoinedGroup, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeGroup, nil, false
	}

	chatID := projectChatID(evt.JID, pctx)
	if chatID == "" {
		return TypeGroup, nil, false
	}

	data := GroupEvent{
		ID: chatID,
	}

	// Resolve sender
	senderJID := resolveSenderJID(evt.Sender, evt.SenderPN)
	senderAlt := resolveSenderJID(evt.SenderPN, evt.Sender)
	if !senderJID.IsEmpty() {
		data.Sender = resolveSender(senderJID, false, senderAlt, pctx)
	}

	// Join reason (e.g., "invite")
	if evt.Reason != "" {
		data.JoinReason = evt.Reason
	}

	// Group type (e.g., "new" for newly created groups)
	if evt.Type != "" {
		data.GroupType = evt.Type
	}

	// Add the current user as joined member
	if pctx.Client != nil && pctx.Client.Store != nil && pctx.Client.Store.ID != nil {
		selfIdentity := resolveIdentityWithAlt(*pctx.Client.Store.ID, pctx.Client.Store.GetLID(), pctx)
		data.Join = []identity.Identity{selfIdentity}
	}

	// Community context fields from embedded GroupInfo
	if evt.IsParent {
		data.IsCommunity = true
	}

	if !evt.LinkedParentJID.IsEmpty() {
		data.CommunityID = evt.LinkedParentJID.String()
	}

	if evt.IsDefaultSubGroup {
		data.IsAnnouncementGroup = true
	}

	return TypeGroup, data, true
}

// projectGroupEphemeral handles the ephemeral setting change within a GroupInfo event.
// It returns a ChatSettingEvent instead of a GroupEvent.
func projectGroupEphemeral(evt *waEvents.GroupInfo, pctx *ProjectorContext) (string, any, bool) {
	ephemeralExpiration := GetEphemeralExpirationString(evt.Ephemeral.DisappearingTimer)

	senderJID := resolveSenderJID(evt.Sender, evt.SenderPN)
	senderAlt := resolveSenderJID(evt.SenderPN, evt.Sender)

	data := ChatSettingEvent{
		ID:          projectChatID(evt.JID, pctx),
		SettingType: "ephemeral",
		Ephemeral: &ChatEphemeralSetting{
			Expiration: ephemeralExpiration,
			Sender:     resolveSender(senderJID, false, senderAlt, pctx),
		},
	}

	return TypeChatSetting, data, true
}

// resolveSenderJID picks the best sender JID from nullable Sender and SenderPN pointers.
func resolveSenderJID(sender, senderPN *waTypes.JID) waTypes.JID {
	if sender != nil {
		return *sender
	}
	if senderPN != nil {
		return *senderPN
	}
	return waTypes.EmptyJID
}
