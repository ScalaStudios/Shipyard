package httpapi

import (
	"context"
	"errors"
	"net/http"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/discord"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/rbac"
)

type discordRequest struct {
	Name       string   `json:"name"`
	Mode       string   `json:"mode"`
	WebhookURL string   `json:"webhook_url"`
	BotToken   string   `json:"bot_token"`
	ChannelID  string   `json:"channel_id"`
	ProjectID  string   `json:"project_id"`
	NotifyOn   []string `json:"notify_on"`
}

func (s *Server) handleListDiscord(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	if _, _, err := s.orgs.Require(r.Context(), currentUser(r).ID, orgID, rbac.PermOrgRead); err != nil {
		mapIdentityError(w, err)
		return
	}
	projectID := r.URL.Query().Get("project_id")
	list, err := s.discord.List(r.Context(), orgID, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = []discord.Integration{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"integrations": list})
}

func (s *Server) handleCreateDiscord(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	if _, _, err := s.orgs.Require(r.Context(), currentUser(r).ID, orgID, rbac.PermOrgUpdate); err != nil {
		mapIdentityError(w, err)
		return
	}
	var req discordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	row, err := s.discord.Create(r.Context(), discord.CreateInput{
		OrganizationID: orgID,
		ProjectID:      req.ProjectID,
		Name:           req.Name,
		Mode:           req.Mode,
		WebhookURL:     req.WebhookURL,
		BotToken:       req.BotToken,
		ChannelID:      req.ChannelID,
		NotifyOn:       req.NotifyOn,
		ActorID:        currentUser(r).ID,
	})
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"integration": row})
}

func (s *Server) handleDeleteDiscord(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	if _, _, err := s.orgs.Require(r.Context(), currentUser(r).ID, orgID, rbac.PermOrgUpdate); err != nil {
		mapIdentityError(w, err)
		return
	}
	if err := s.discord.Delete(r.Context(), orgID, r.PathValue("integrationID")); err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

func (s *Server) handleTestDiscord(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	if _, _, err := s.orgs.Require(r.Context(), currentUser(r).ID, orgID, rbac.PermOrgUpdate); err != nil {
		mapIdentityError(w, err)
		return
	}
	if err := s.discord.TestPing(r.Context(), orgID, r.PathValue("integrationID")); err != nil {
		if errors.Is(err, identity.ErrNotFound) || errors.Is(err, identity.ErrInvalidInput) {
			mapIdentityError(w, err)
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "sent"})
}

func (s *Server) fanoutNotify(ctx context.Context, orgID, projectID, kind, title, body, href string, number int64, status, ref, sha, projectSlug string) {
	_ = s.notifications.NotifyOrgMembers(ctx, orgID, projectID, kind, title, body, href)
	if s.discord != nil {
		_ = s.discord.Notify(ctx, orgID, projectID, discord.Event{
			Kind: kind, Title: title, Body: body, Href: href,
			Number: number, Status: status, Ref: ref, SHA: sha, Project: projectSlug,
		})
	}
}
