package handler

import (
	"encoding/base64"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type GroupHandler struct {
	Handler
}

func NewGroupHandler(logger *slog.Logger) *GroupHandler {
	return &GroupHandler{Handler: Handler{Logger: logger}}
}

func (h *GroupHandler) RegisterRoutes(r chi.Router) {
	r.Route("/groups", func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/{id}", h.Get)
		r.Put("/{id}/name", h.SetName)
		r.Put("/{id}/description", h.SetDescription)
		r.Post("/{id}/picture", h.SetPicture)
		r.Post("/{id}/leave", h.Leave)
		r.Get("/{id}/participants", h.GetParticipants)
		r.Put("/{id}/participants", h.UpdateParticipants)
		r.Get("/{id}/invite-link", h.GetInviteLink)
		r.Post("/{id}/invite-link/reset", h.ResetInviteLink)
		r.Put("/{id}/settings/announce", h.SetAnnounce)
		r.Put("/{id}/settings/locked", h.SetLocked)
		r.Put("/{id}/settings/join-approval", h.SetJoinApproval)
		r.Put("/{id}/settings/member-add-mode", h.SetMemberAddMode)
		r.Post("/join/link", h.JoinWithLink)
		r.Post("/join/invite", h.JoinWithInvite)
		r.Get("/invite/{code}", h.GetInfoFromLink)
		r.Get("/{id}/requests", h.GetRequests)
		r.Put("/{id}/requests", h.UpdateRequests)
	})
}

type CreateGroupRequest struct {
	Name         string   `json:"name" validate:"required"`
	Participants []string `json:"participants" validate:"required,min=1"`
}

type UpdateParticipantsRequest struct {
	Participants []string `json:"participants" validate:"required,min=1"`
	Action       string   `json:"action" validate:"required,oneof=add remove promote demote"`
}

type SetNameRequest struct {
	Name string `json:"name" validate:"required"`
}

type SetDescriptionRequest struct {
	Description string `json:"description"`
}

type SetPictureRequest struct {
	Data string `json:"data" validate:"required"`
}

type SetBoolRequest struct {
	Enabled bool `json:"enabled"`
}

type SetMemberAddModeRequest struct {
	OnlyAdminAdd bool `json:"onlyAdminAdd"`
}

type JoinWithLinkRequest struct {
	Code string `json:"code" validate:"required"`
}

type JoinWithInviteRequest struct {
	GroupID    string `json:"groupId" validate:"required"`
	InviterID  string `json:"inviterId" validate:"required"`
	Code       string `json:"code" validate:"required"`
	Expiration int64  `json:"expiration"`
}

type UpdateRequestsRequest struct {
	Participants []string `json:"participants" validate:"required,min=1"`
	Action       string   `json:"action" validate:"required,oneof=approve reject"`
}

func (h *GroupHandler) List(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	groups, err := inst.Service.Groups.GetJoinedGroups(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, groups)
}

func (h *GroupHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateGroupRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	id, err := inst.Service.Groups.CreateGroup(r.Context(), req.Name, req.Participants)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}

func (h *GroupHandler) Get(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	groupID := chi.URLParam(r, "id")
	info, err := inst.Service.Groups.GetGroupInfo(r.Context(), groupID)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, info)
}

func (h *GroupHandler) SetName(w http.ResponseWriter, r *http.Request) {
	var req SetNameRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Groups.SetGroupName(r.Context(), chi.URLParam(r, "id"), req.Name); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *GroupHandler) SetDescription(w http.ResponseWriter, r *http.Request) {
	var req SetDescriptionRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Groups.SetGroupDescription(r.Context(), chi.URLParam(r, "id"), req.Description); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *GroupHandler) SetPicture(w http.ResponseWriter, r *http.Request) {
	var req SetPictureRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		h.Error(w, "invalid base64 data", http.StatusBadRequest)
		return
	}
	id, err := inst.Service.Groups.SetGroupPicture(r.Context(), chi.URLParam(r, "id"), data)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, map[string]string{"pictureId": id})
}

func (h *GroupHandler) Leave(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	if err := inst.Service.Groups.LeaveGroup(r.Context(), chi.URLParam(r, "id")); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *GroupHandler) GetParticipants(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	participants, err := inst.Service.Groups.GetGroupParticipants(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, participants)
}

func (h *GroupHandler) UpdateParticipants(w http.ResponseWriter, r *http.Request) {
	var req UpdateParticipantsRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Groups.UpdateGroupParticipants(r.Context(), chi.URLParam(r, "id"), req.Participants, req.Action); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *GroupHandler) GetInviteLink(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	link, err := inst.Service.Groups.GetGroupInviteLink(r.Context(), chi.URLParam(r, "id"), false)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, map[string]string{"link": link})
}

func (h *GroupHandler) ResetInviteLink(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	link, err := inst.Service.Groups.GetGroupInviteLink(r.Context(), chi.URLParam(r, "id"), true)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, map[string]string{"link": link})
}

func (h *GroupHandler) SetAnnounce(w http.ResponseWriter, r *http.Request) {
	var req SetBoolRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Groups.SetGroupAnnounceMode(r.Context(), chi.URLParam(r, "id"), req.Enabled); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *GroupHandler) SetLocked(w http.ResponseWriter, r *http.Request) {
	var req SetBoolRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Groups.SetGroupLocked(r.Context(), chi.URLParam(r, "id"), req.Enabled); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *GroupHandler) SetJoinApproval(w http.ResponseWriter, r *http.Request) {
	var req SetBoolRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Groups.SetGroupJoinApprovalMode(r.Context(), chi.URLParam(r, "id"), req.Enabled); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *GroupHandler) SetMemberAddMode(w http.ResponseWriter, r *http.Request) {
	var req SetMemberAddModeRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Groups.SetGroupMemberAddMode(r.Context(), chi.URLParam(r, "id"), req.OnlyAdminAdd); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *GroupHandler) JoinWithLink(w http.ResponseWriter, r *http.Request) {
	var req JoinWithLinkRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	id, err := inst.Service.Groups.JoinGroupWithLink(r.Context(), req.Code)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}

func (h *GroupHandler) JoinWithInvite(w http.ResponseWriter, r *http.Request) {
	var req JoinWithInviteRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Groups.JoinGroupWithInvite(r.Context(), req.GroupID, req.InviterID, req.Code, req.Expiration); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *GroupHandler) GetInfoFromLink(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	code := chi.URLParam(r, "code")
	info, err := inst.Service.Groups.GetGroupInfoFromLink(r.Context(), code)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, info)
}

func (h *GroupHandler) GetRequests(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	requests, err := inst.Service.Groups.GetGroupRequestParticipants(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, requests)
}

func (h *GroupHandler) UpdateRequests(w http.ResponseWriter, r *http.Request) {
	var req UpdateRequestsRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Groups.UpdateGroupRequestParticipants(r.Context(), chi.URLParam(r, "id"), req.Participants, req.Action); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}
