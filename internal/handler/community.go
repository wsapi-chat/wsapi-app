package handler

import (
	"encoding/base64"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type CommunityHandler struct {
	Handler
}

func NewCommunityHandler(logger *slog.Logger) *CommunityHandler {
	return &CommunityHandler{Handler: Handler{Logger: logger}}
}

func (h *CommunityHandler) RegisterRoutes(r chi.Router) {
	r.Route("/communities", func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/{id}", h.Get)
		r.Post("/{id}/leave", h.Leave)
		r.Put("/{id}/name", h.SetName)
		r.Put("/{id}/description", h.SetDescription)
		r.Post("/{id}/picture", h.SetPicture)
		r.Put("/{id}/settings/locked", h.SetLocked)
		r.Put("/{id}/participants", h.UpdateParticipants)
		r.Get("/{id}/participants", h.GetParticipants)
		r.Get("/{id}/invite-link", h.GetInviteLink)
		r.Post("/{id}/invite-link/reset", h.ResetInviteLink)
		r.Get("/{id}/groups", h.GetSubGroups)
		r.Post("/{id}/groups", h.CreateGroup)
		r.Post("/{id}/groups/link", h.LinkGroup)
		r.Delete("/{id}/groups/{groupId}", h.UnlinkGroup)
	})
}

type CreateCommunityRequest struct {
	Name         string   `json:"name" validate:"required"`
	Participants []string `json:"participants,omitempty"`
	ApprovalMode string   `json:"approvalMode,omitempty"`
}

type CreateCommunityGroupRequest struct {
	Name         string   `json:"name" validate:"required"`
	Participants []string `json:"participants,omitempty"`
}

type LinkGroupRequest struct {
	GroupID string `json:"groupId" validate:"required"`
}

func (h *CommunityHandler) List(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	communities, err := inst.Service.Communities.GetJoinedCommunities(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, communities)
}

func (h *CommunityHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateCommunityRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	id, err := inst.Service.Communities.CreateCommunity(r.Context(), req.Name, req.Participants, req.ApprovalMode)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}

func (h *CommunityHandler) Get(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	info, err := inst.Service.Communities.GetCommunityInfo(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, info)
}

func (h *CommunityHandler) Leave(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	if err := inst.Service.Communities.LeaveCommunity(r.Context(), chi.URLParam(r, "id")); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *CommunityHandler) SetName(w http.ResponseWriter, r *http.Request) {
	var req SetNameRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Communities.SetCommunityName(r.Context(), chi.URLParam(r, "id"), req.Name); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *CommunityHandler) SetDescription(w http.ResponseWriter, r *http.Request) {
	var req SetDescriptionRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Communities.SetCommunityDescription(r.Context(), chi.URLParam(r, "id"), req.Description); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *CommunityHandler) SetPicture(w http.ResponseWriter, r *http.Request) {
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
	id, err := inst.Service.Communities.SetCommunityPicture(r.Context(), chi.URLParam(r, "id"), data)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, map[string]string{"pictureId": id})
}

func (h *CommunityHandler) SetLocked(w http.ResponseWriter, r *http.Request) {
	var req SetBoolRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Communities.SetCommunityLocked(r.Context(), chi.URLParam(r, "id"), req.Enabled); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *CommunityHandler) UpdateParticipants(w http.ResponseWriter, r *http.Request) {
	var req UpdateParticipantsRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Communities.UpdateCommunityParticipants(r.Context(), chi.URLParam(r, "id"), req.Participants, req.Action); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *CommunityHandler) GetParticipants(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	participants, err := inst.Service.Communities.GetCommunityParticipants(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, participants)
}

func (h *CommunityHandler) GetInviteLink(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	link, err := inst.Service.Communities.GetCommunityInviteLink(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, map[string]string{"link": link})
}

func (h *CommunityHandler) ResetInviteLink(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	link, err := inst.Service.Communities.ResetCommunityInviteLink(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, map[string]string{"link": link})
}

func (h *CommunityHandler) GetSubGroups(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	groups, err := inst.Service.Communities.GetCommunitySubGroups(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, groups)
}

func (h *CommunityHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req CreateCommunityGroupRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	id, err := inst.Service.Communities.CreateCommunityGroup(r.Context(), chi.URLParam(r, "id"), req.Name, req.Participants)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}

func (h *CommunityHandler) LinkGroup(w http.ResponseWriter, r *http.Request) {
	var req LinkGroupRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Communities.LinkGroupToCommunity(r.Context(), chi.URLParam(r, "id"), req.GroupID); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *CommunityHandler) UnlinkGroup(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	communityID := chi.URLParam(r, "id")
	groupID := chi.URLParam(r, "groupId")
	if err := inst.Service.Communities.UnlinkGroupFromCommunity(r.Context(), communityID, groupID); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}
