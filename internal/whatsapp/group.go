package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wsapi-chat/wsapi-app/internal/identity"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	waTypes "go.mau.fi/whatsmeow/types"
)

// GroupInfoResponse is the domain response type for group information.
type GroupInfoResponse struct {
	GroupID                string             `json:"groupId"`
	Owner                  identity.Identity  `json:"owner"`
	Name                   string             `json:"name"`
	CreatedAt              time.Time          `json:"createdAt"`
	Description            string             `json:"description"`
	IsAnnounce             bool               `json:"isAnnounce"`
	IsLocked               bool               `json:"isLocked"`
	IsEphemeral            bool               `json:"isEphemeral"`
	EphemeralExpiration    int64              `json:"ephemeralExpiration"`
	Participants           []GroupParticipant `json:"participants"`
	CommunityID            string             `json:"communityId,omitempty"`
	IsAnnouncementGroup    bool               `json:"isAnnouncementGroup,omitempty"`
	IsJoinApprovalRequired bool               `json:"isJoinApprovalRequired"`
	MemberAddMode          string             `json:"memberAddMode"`
}

// GroupParticipant represents a participant in a group or community.
type GroupParticipant struct {
	identity.Identity
	IsAdmin      bool   `json:"isAdmin"`
	IsSuperAdmin bool   `json:"isSuperAdmin"`
	DisplayName  string `json:"displayName"`
}

// GroupParticipantRequest represents a pending join request.
type GroupParticipantRequest struct {
	User        identity.Identity `json:"user"`
	RequestedAt time.Time         `json:"requestedAt"`
}

// GroupService wraps the whatsmeow client for group operations.
type GroupService struct {
	client *whatsmeow.Client
	logger *slog.Logger
}

// CreateGroup creates a new WhatsApp group with the given name and participants.
func (g *GroupService) CreateGroup(ctx context.Context, name string, participants []string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("group name cannot be empty")
	}
	if len(participants) == 0 {
		return "", fmt.Errorf("at least one participant is required")
	}

	waJIDs := make([]waTypes.JID, 0, len(participants))
	for _, p := range participants {
		jid, err := waTypes.ParseJID(p)
		if err != nil {
			return "", fmt.Errorf("invalid participant ID '%s': %w", p, err)
		}
		waJIDs = append(waJIDs, jid)
	}

	req := whatsmeow.ReqCreateGroup{
		Name:         name,
		Participants: waJIDs,
	}

	groupInfo, err := g.client.CreateGroup(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create group: %w", err)
	}
	return groupInfo.JID.String(), nil
}

// UpdateGroupParticipants adds, removes, promotes, or demotes participants.
func (g *GroupService) UpdateGroupParticipants(ctx context.Context, groupID string, participants []string, action string) error {
	jid, err := waTypes.ParseJID(groupID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}

	waParticipants := make([]waTypes.JID, 0, len(participants))
	for _, p := range participants {
		pJID, err := waTypes.ParseJID(p)
		if err != nil {
			return fmt.Errorf("invalid participant JID: %w", err)
		}
		waParticipants = append(waParticipants, pJID)
	}

	var waAction whatsmeow.ParticipantChange
	switch action {
	case "add":
		waAction = whatsmeow.ParticipantChangeAdd
	case "remove":
		waAction = whatsmeow.ParticipantChangeRemove
	case "promote":
		waAction = whatsmeow.ParticipantChangePromote
	case "demote":
		waAction = whatsmeow.ParticipantChangeDemote
	default:
		return fmt.Errorf("invalid action: %s", action)
	}

	_, err = g.client.UpdateGroupParticipants(ctx, jid, waParticipants, waAction)
	return err
}

// GetGroupInfo returns information about a group.
func (g *GroupService) GetGroupInfo(ctx context.Context, groupID string) (GroupInfoResponse, error) {
	jid, err := waTypes.ParseJID(groupID)
	if err != nil {
		return GroupInfoResponse{}, fmt.Errorf("invalid group JID: %w", err)
	}
	info, err := g.client.GetGroupInfo(ctx, jid)
	if err != nil {
		return GroupInfoResponse{}, fmt.Errorf("%w: %v", ErrNotFound, err)
	}
	return toGroupInfoResponse(ctx, info, g.client.Store.LIDs), nil
}

// GetGroupParticipants returns just the participants for a group.
func (g *GroupService) GetGroupParticipants(ctx context.Context, groupID string) ([]GroupParticipant, error) {
	jid, err := waTypes.ParseJID(groupID)
	if err != nil {
		return nil, fmt.Errorf("invalid group JID: %w", err)
	}
	info, err := g.client.GetGroupInfo(ctx, jid)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotFound, err)
	}
	resp := toGroupInfoResponse(ctx, info, g.client.Store.LIDs)
	return resp.Participants, nil
}

// GetGroupInfoFromLink resolves an invite link and returns group info.
func (g *GroupService) GetGroupInfoFromLink(ctx context.Context, code string) (GroupInfoResponse, error) {
	info, err := g.client.GetGroupInfoFromLink(ctx, code)
	if err != nil {
		return GroupInfoResponse{}, fmt.Errorf("%w: %v", ErrNotFound, err)
	}
	return toGroupInfoResponse(ctx, info, g.client.Store.LIDs), nil
}

// GetGroupInfoFromInvite gets the group info from an invite message.
func (g *GroupService) GetGroupInfoFromInvite(ctx context.Context, groupID, inviterID, code string, expiration int64) (GroupInfoResponse, error) {
	jid, err := waTypes.ParseJID(groupID)
	if err != nil {
		return GroupInfoResponse{}, fmt.Errorf("invalid group JID: %w", err)
	}
	inviter, err := waTypes.ParseJID(inviterID)
	if err != nil {
		return GroupInfoResponse{}, fmt.Errorf("invalid inviter JID: %w", err)
	}
	info, err := g.client.GetGroupInfoFromInvite(ctx, jid, inviter, code, expiration)
	if err != nil {
		return GroupInfoResponse{}, fmt.Errorf("%w: %v", ErrNotFound, err)
	}
	return toGroupInfoResponse(ctx, info, g.client.Store.LIDs), nil
}

// GetGroupInviteLink returns or resets the group invite link.
func (g *GroupService) GetGroupInviteLink(ctx context.Context, groupID string, reset bool) (string, error) {
	jid, err := waTypes.ParseJID(groupID)
	if err != nil {
		return "", fmt.Errorf("invalid group JID: %w", err)
	}
	return g.client.GetGroupInviteLink(ctx, jid, reset)
}

// GetJoinedGroups returns the list of groups the user is participating in.
func (g *GroupService) GetJoinedGroups(ctx context.Context) ([]GroupInfoResponse, error) {
	info, err := g.client.GetJoinedGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get joined groups: %w", err)
	}
	result := make([]GroupInfoResponse, 0, len(info))
	for _, gi := range info {
		result = append(result, toGroupInfoResponse(ctx, gi, g.client.Store.LIDs))
	}
	return result, nil
}

// JoinGroupWithInvite joins a group using an invite message.
func (g *GroupService) JoinGroupWithInvite(ctx context.Context, groupID, inviterID, code string, expiration int64) error {
	jid, err := waTypes.ParseJID(groupID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}
	inviter, err := waTypes.ParseJID(inviterID)
	if err != nil {
		return fmt.Errorf("invalid inviter JID: %w", err)
	}
	return g.client.JoinGroupWithInvite(ctx, jid, inviter, code, expiration)
}

// JoinGroupWithLink joins a group using an invite link code.
func (g *GroupService) JoinGroupWithLink(ctx context.Context, code string) (string, error) {
	jid, err := g.client.JoinGroupWithLink(ctx, code)
	if err != nil {
		return "", fmt.Errorf("failed to join group: %w", err)
	}
	return jid.String(), nil
}

// LeaveGroup leaves the specified group.
func (g *GroupService) LeaveGroup(ctx context.Context, groupID string) error {
	jid, err := waTypes.ParseJID(groupID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}
	return g.client.LeaveGroup(ctx, jid)
}

// SetGroupAnnounceMode sets the announce mode for a group.
func (g *GroupService) SetGroupAnnounceMode(ctx context.Context, groupID string, announceMode bool) error {
	jid, err := waTypes.ParseJID(groupID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}
	return g.client.SetGroupAnnounce(ctx, jid, announceMode)
}

// SetGroupDescription sets the description for a group.
func (g *GroupService) SetGroupDescription(ctx context.Context, groupID string, description string) error {
	jid, err := waTypes.ParseJID(groupID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}
	return g.client.SetGroupDescription(ctx, jid, description)
}

// SetGroupJoinApprovalMode sets the join approval mode for a group.
func (g *GroupService) SetGroupJoinApprovalMode(ctx context.Context, groupID string, joinApprovalMode bool) error {
	jid, err := waTypes.ParseJID(groupID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}
	return g.client.SetGroupJoinApprovalMode(ctx, jid, joinApprovalMode)
}

// SetGroupLocked sets the locked mode for a group.
func (g *GroupService) SetGroupLocked(ctx context.Context, groupID string, locked bool) error {
	jid, err := waTypes.ParseJID(groupID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}
	return g.client.SetGroupLocked(ctx, jid, locked)
}

// SetGroupMemberAddMode sets the member add mode for a group.
func (g *GroupService) SetGroupMemberAddMode(ctx context.Context, groupID string, onlyAdminAdd bool) error {
	jid, err := waTypes.ParseJID(groupID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}

	waMemberAddMode := waTypes.GroupMemberAddModeAllMember
	if onlyAdminAdd {
		waMemberAddMode = waTypes.GroupMemberAddModeAdmin
	}

	return g.client.SetGroupMemberAddMode(ctx, jid, waMemberAddMode)
}

// SetGroupName sets the name for a group.
func (g *GroupService) SetGroupName(ctx context.Context, groupID string, name string) error {
	jid, err := waTypes.ParseJID(groupID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}
	return g.client.SetGroupName(ctx, jid, name)
}

// SetGroupPicture sets the photo for a group.
func (g *GroupService) SetGroupPicture(ctx context.Context, groupID string, photo []byte) (string, error) {
	jid, err := waTypes.ParseJID(groupID)
	if err != nil {
		return "", fmt.Errorf("invalid group JID: %w", err)
	}
	photo, err = ResizeImageIfNeeded(photo)
	if err != nil {
		return "", fmt.Errorf("failed to resize image: %w", err)
	}
	return g.client.SetGroupPhoto(ctx, jid, photo)
}

// GetGroupRequestParticipants returns the list of pending join requests.
func (g *GroupService) GetGroupRequestParticipants(ctx context.Context, groupID string) ([]GroupParticipantRequest, error) {
	jid, err := waTypes.ParseJID(groupID)
	if err != nil {
		return nil, fmt.Errorf("invalid group JID: %w", err)
	}
	requests, err := g.client.GetGroupRequestParticipants(ctx, jid)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotFound, err)
	}
	lids := g.client.Store.LIDs
	result := make([]GroupParticipantRequest, 0, len(requests))
	for _, request := range requests {
		result = append(result, GroupParticipantRequest{
			User:        identity.Resolve(ctx, request.JID, waTypes.EmptyJID, lids),
			RequestedAt: request.RequestedAt,
		})
	}
	return result, nil
}

// UpdateGroupRequestParticipants approves or rejects pending join requests.
func (g *GroupService) UpdateGroupRequestParticipants(ctx context.Context, groupID string, participants []string, action string) error {
	jid, err := waTypes.ParseJID(groupID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}
	waParticipants := make([]waTypes.JID, 0, len(participants))
	for _, p := range participants {
		pJID, err := waTypes.ParseJID(p)
		if err != nil {
			return fmt.Errorf("invalid participant JID: %w", err)
		}
		waParticipants = append(waParticipants, pJID)
	}

	var waAction whatsmeow.ParticipantRequestChange
	switch action {
	case "approve":
		waAction = whatsmeow.ParticipantChangeApprove
	case "reject":
		waAction = whatsmeow.ParticipantChangeReject
	default:
		return fmt.Errorf("invalid action: %s", action)
	}

	_, err = g.client.UpdateGroupRequestParticipants(ctx, jid, waParticipants, waAction)
	return err
}

// toGroupInfoResponse converts a whatsmeow GroupInfo to the domain GroupInfoResponse.
func toGroupInfoResponse(ctx context.Context, info *waTypes.GroupInfo, lids store.LIDStore) GroupInfoResponse {
	if info == nil {
		return GroupInfoResponse{}
	}

	participants := make([]GroupParticipant, 0, len(info.Participants))
	for _, p := range info.Participants {
		// Use PhoneNumber and LID from the participant to build the alt JID.
		var altJID waTypes.JID
		if p.JID.Server == waTypes.HiddenUserServer && !p.PhoneNumber.IsEmpty() {
			altJID = p.PhoneNumber
		} else if p.JID.Server == waTypes.DefaultUserServer && !p.LID.IsEmpty() {
			altJID = p.LID
		}

		participants = append(participants, GroupParticipant{
			Identity:     identity.Resolve(ctx, p.JID, altJID, lids),
			IsAdmin:      p.IsAdmin,
			IsSuperAdmin: p.IsSuperAdmin,
			DisplayName:  p.DisplayName,
		})
	}

	// Resolve owner: use OwnerPN as primary (phone-based), OwnerJID as alt.
	var ownerJID, ownerAlt waTypes.JID
	if !info.OwnerPN.IsEmpty() {
		ownerJID = info.OwnerPN
		ownerAlt = info.OwnerJID
	} else {
		ownerJID = info.OwnerJID
	}

	communityID := ""
	if !info.LinkedParentJID.IsEmpty() {
		communityID = info.LinkedParentJID.String()
	}

	return GroupInfoResponse{
		GroupID:                info.JID.String(),
		Owner:                  identity.Resolve(ctx, ownerJID, ownerAlt, lids),
		Name:                   info.Name,
		CreatedAt:              info.GroupCreated,
		Description:            info.Topic,
		IsAnnounce:             info.IsAnnounce,
		IsLocked:               info.IsLocked,
		IsEphemeral:            info.IsEphemeral,
		EphemeralExpiration:    int64(info.DisappearingTimer),
		Participants:           participants,
		CommunityID:            communityID,
		IsAnnouncementGroup:    info.IsDefaultSubGroup,
		IsJoinApprovalRequired: info.IsJoinApprovalRequired,
		MemberAddMode:          string(info.MemberAddMode),
	}
}
