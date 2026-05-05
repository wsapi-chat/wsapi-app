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

// CommunityInfoResponse is the domain response type for community information.
type CommunityInfoResponse struct {
	CommunityID           string             `json:"communityId"`
	Owner                 identity.Identity  `json:"owner"`
	Name                  string             `json:"name"`
	Created               time.Time          `json:"created"`
	Description           string             `json:"description"`
	IsLocked              bool               `json:"isLocked"`
	CommunityApprovalMode string             `json:"communityApprovalMode"`
	Participants          []GroupParticipant `json:"participants"`
}

// CommunitySubGroupResponse represents a sub-group within a community.
type CommunitySubGroupResponse struct {
	GroupID             string    `json:"groupId"`
	Name                string    `json:"name"`
	NameSetAt           time.Time `json:"nameSetAt"`
	IsAnnouncementGroup bool      `json:"isAnnouncementGroup"`
}

// CommunityService wraps the whatsmeow client for community operations.
type CommunityService struct {
	client *whatsmeow.Client
	logger *slog.Logger
}

// CreateCommunity creates a new WhatsApp community.
func (c *CommunityService) CreateCommunity(ctx context.Context, name string, participants []string, approvalMode string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("community name cannot be empty")
	}

	waJIDs := make([]waTypes.JID, 0, len(participants))
	for _, p := range participants {
		if p == "" {
			return "", fmt.Errorf("participant ID cannot be empty")
		}
		waJIDs = append(waJIDs, FormatRecipient(p))
	}

	req := whatsmeow.ReqCreateGroup{
		Name:         name,
		Participants: waJIDs,
	}
	req.IsParent = true
	if approvalMode != "" {
		req.DefaultMembershipApprovalMode = approvalMode
	}

	groupInfo, err := c.client.CreateGroup(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create community: %w", err)
	}
	return groupInfo.JID.String(), nil
}

// GetJoinedCommunities returns all communities the user has joined.
func (c *CommunityService) GetJoinedCommunities(ctx context.Context) ([]CommunityInfoResponse, error) {
	groups, err := c.client.GetJoinedGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get joined groups: %w", err)
	}

	communities := make([]CommunityInfoResponse, 0)
	lids := c.client.Store.LIDs
	for _, group := range groups {
		if group.IsParent {
			communities = append(communities, toCommunityInfoResponse(ctx, group, lids))
		}
	}
	return communities, nil
}

// GetCommunityInfo returns info about a specific community.
func (c *CommunityService) GetCommunityInfo(ctx context.Context, communityID string) (CommunityInfoResponse, error) {
	jid, err := parseJID(communityID)
	if err != nil {
		return CommunityInfoResponse{}, fmt.Errorf("invalid community JID: %w", err)
	}

	info, err := c.client.GetGroupInfo(ctx, jid)
	if err != nil {
		return CommunityInfoResponse{}, fmt.Errorf("%w: %v", ErrNotFound, err)
	}

	if !info.IsParent {
		return CommunityInfoResponse{}, fmt.Errorf("%w: JID is not a community", ErrNotFound)
	}

	return toCommunityInfoResponse(ctx, info, c.client.Store.LIDs), nil
}

// LeaveCommunity leaves the specified community.
func (c *CommunityService) LeaveCommunity(ctx context.Context, communityID string) error {
	jid, err := parseJID(communityID)
	if err != nil {
		return fmt.Errorf("invalid community JID: %w", err)
	}
	return c.client.LeaveGroup(ctx, jid)
}

// SetCommunityName sets the name of a community.
func (c *CommunityService) SetCommunityName(ctx context.Context, communityID string, name string) error {
	jid, err := parseJID(communityID)
	if err != nil {
		return fmt.Errorf("invalid community JID: %w", err)
	}
	return c.client.SetGroupName(ctx, jid, name)
}

// SetCommunityDescription sets the description of a community.
func (c *CommunityService) SetCommunityDescription(ctx context.Context, communityID string, description string) error {
	jid, err := parseJID(communityID)
	if err != nil {
		return fmt.Errorf("invalid community JID: %w", err)
	}
	return c.client.SetGroupDescription(ctx, jid, description)
}

// SetCommunityPicture sets the picture of a community.
func (c *CommunityService) SetCommunityPicture(ctx context.Context, communityID string, photo []byte) (string, error) {
	jid, err := parseJID(communityID)
	if err != nil {
		return "", fmt.Errorf("invalid community JID: %w", err)
	}
	photo, err = ResizeImageIfNeeded(photo)
	if err != nil {
		return "", fmt.Errorf("failed to resize image: %w", err)
	}
	return c.client.SetGroupPhoto(ctx, jid, photo)
}

// SetCommunityLocked sets the locked mode for a community.
func (c *CommunityService) SetCommunityLocked(ctx context.Context, communityID string, locked bool) error {
	jid, err := parseJID(communityID)
	if err != nil {
		return fmt.Errorf("invalid community JID: %w", err)
	}
	return c.client.SetGroupLocked(ctx, jid, locked)
}

// GetCommunityParticipants returns all participants across all community groups.
func (c *CommunityService) GetCommunityParticipants(ctx context.Context, communityID string) ([]identity.Identity, error) {
	jid, err := parseJID(communityID)
	if err != nil {
		return nil, fmt.Errorf("invalid community JID: %w", err)
	}

	participants, err := c.client.GetLinkedGroupsParticipants(ctx, jid)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotFound, err)
	}

	lids := c.client.Store.LIDs
	result := make([]identity.Identity, 0, len(participants))
	for _, p := range participants {
		result = append(result, identity.Resolve(ctx, p, waTypes.EmptyJID, lids))
	}
	return result, nil
}

// UpdateCommunityParticipants updates participants in a community.
func (c *CommunityService) UpdateCommunityParticipants(ctx context.Context, communityID string, participants []string, action string) error {
	jid, err := parseJID(communityID)
	if err != nil {
		return fmt.Errorf("invalid community JID: %w", err)
	}

	waParticipants := make([]waTypes.JID, 0, len(participants))
	for _, p := range participants {
		if p == "" {
			return fmt.Errorf("participant ID cannot be empty")
		}
		waParticipants = append(waParticipants, FormatRecipient(p))
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

	_, err = c.client.UpdateGroupParticipants(ctx, jid, waParticipants, waAction)
	return err
}

// GetCommunityInviteLink returns the invite link for a community.
func (c *CommunityService) GetCommunityInviteLink(ctx context.Context, communityID string) (string, error) {
	jid, err := parseJID(communityID)
	if err != nil {
		return "", fmt.Errorf("invalid community JID: %w", err)
	}
	return c.client.GetGroupInviteLink(ctx, jid, false)
}

// ResetCommunityInviteLink resets and returns a new invite link for a community.
func (c *CommunityService) ResetCommunityInviteLink(ctx context.Context, communityID string) (string, error) {
	jid, err := parseJID(communityID)
	if err != nil {
		return "", fmt.Errorf("invalid community JID: %w", err)
	}
	return c.client.GetGroupInviteLink(ctx, jid, true)
}

// GetCommunitySubGroups returns all groups within a community.
func (c *CommunityService) GetCommunitySubGroups(ctx context.Context, communityID string) ([]CommunitySubGroupResponse, error) {
	jid, err := parseJID(communityID)
	if err != nil {
		return nil, fmt.Errorf("invalid community JID: %w", err)
	}

	subGroups, err := c.client.GetSubGroups(ctx, jid)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotFound, err)
	}

	result := make([]CommunitySubGroupResponse, 0, len(subGroups))
	for _, sg := range subGroups {
		result = append(result, CommunitySubGroupResponse{
			GroupID:             sg.JID.String(),
			Name:                sg.Name,
			NameSetAt:           sg.NameSetAt,
			IsAnnouncementGroup: sg.IsDefaultSubGroup,
		})
	}
	return result, nil
}

// CreateCommunityGroup creates a new group within a community.
func (c *CommunityService) CreateCommunityGroup(ctx context.Context, communityID string, name string, participants []string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("group name cannot be empty")
	}

	communityJID, err := parseJID(communityID)
	if err != nil {
		return "", fmt.Errorf("invalid community JID: %w", err)
	}

	waJIDs := make([]waTypes.JID, 0, len(participants))
	for _, p := range participants {
		if p == "" {
			return "", fmt.Errorf("participant ID cannot be empty")
		}
		waJIDs = append(waJIDs, FormatRecipient(p))
	}

	req := whatsmeow.ReqCreateGroup{
		Name:         name,
		Participants: waJIDs,
	}
	req.LinkedParentJID = communityJID

	groupInfo, err := c.client.CreateGroup(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create group in community: %w", err)
	}
	return groupInfo.JID.String(), nil
}

// LinkGroupToCommunity links an existing group to a community.
func (c *CommunityService) LinkGroupToCommunity(ctx context.Context, communityID string, groupID string) error {
	communityJID, err := parseJID(communityID)
	if err != nil {
		return fmt.Errorf("invalid community JID: %w", err)
	}

	groupJID, err := parseJID(groupID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}

	return c.client.LinkGroup(ctx, communityJID, groupJID)
}

// UnlinkGroupFromCommunity removes a group from a community.
func (c *CommunityService) UnlinkGroupFromCommunity(ctx context.Context, communityID string, groupID string) error {
	communityJID, err := parseJID(communityID)
	if err != nil {
		return fmt.Errorf("invalid community JID: %w", err)
	}

	groupJID, err := parseJID(groupID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}

	return c.client.UnlinkGroup(ctx, communityJID, groupJID)
}

// toCommunityInfoResponse converts a whatsmeow GroupInfo to CommunityInfoResponse.
func toCommunityInfoResponse(ctx context.Context, info *waTypes.GroupInfo, lids store.LIDStore) CommunityInfoResponse {
	if info == nil {
		return CommunityInfoResponse{}
	}

	participants := make([]GroupParticipant, 0, len(info.Participants))
	for _, p := range info.Participants {
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

	// Resolve owner
	var ownerJID, ownerAlt waTypes.JID
	if !info.OwnerPN.IsEmpty() {
		ownerJID = info.OwnerPN
		ownerAlt = info.OwnerJID
	} else {
		ownerJID = info.OwnerJID
	}

	return CommunityInfoResponse{
		CommunityID:           info.JID.String(),
		Owner:                 identity.Resolve(ctx, ownerJID, ownerAlt, lids),
		Name:                  info.Name,
		Created:               info.GroupCreated,
		Description:           info.Topic,
		IsLocked:              info.IsLocked,
		CommunityApprovalMode: info.DefaultMembershipApprovalMode,
		Participants:          participants,
	}
}
