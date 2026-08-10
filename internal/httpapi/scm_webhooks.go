package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/notifications"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/rbac"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/scm"
)

func (s *Server) handleListSCMProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"providers": scm.Catalog})
}

type scmConnectionRequest struct {
	Provider      string `json:"provider"`
	Name          string `json:"name"`
	BaseURL       string `json:"base_url"`
	RepoOwner     string `json:"repo_owner"`
	RepoName      string `json:"repo_name"`
	AccessToken   string `json:"access_token"`
	BotUsername   string `json:"bot_username"`
	WebhookSecret string `json:"webhook_secret"`
	PipelineSlug  string `json:"pipeline_slug"`
}

func (s *Server) handleListSCMConnections(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	list, err := s.scm.List(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = []scm.Connection{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"connections": list})
}

func (s *Server) handleCreateSCMConnection(w http.ResponseWriter, r *http.Request) {
	org, project, ok := s.projectAccess(w, r, rbac.PermProjectUpdate)
	if !ok {
		return
	}
	var req scmConnectionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	conn, err := s.scm.Create(r.Context(), scm.CreateInput{
		OrganizationID: org.ID,
		ProjectID:      project.ID,
		Provider:       req.Provider,
		Name:           req.Name,
		BaseURL:        req.BaseURL,
		RepoOwner:      req.RepoOwner,
		RepoName:       req.RepoName,
		AccessToken:    req.AccessToken,
		BotUsername:    req.BotUsername,
		WebhookSecret:  req.WebhookSecret,
		PipelineSlug:   req.PipelineSlug,
		ActorID:        currentUser(r).ID,
	})
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"connection":   conn,
		"webhook_url":  fmt.Sprintf("/api/v1/webhooks/%s?connection_id=%s", conn.Provider, conn.ID),
	})
}

func (s *Server) handleDeleteSCMConnection(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectUpdate)
	if !ok {
		return
	}
	if err := s.scm.Delete(r.Context(), project.ID, r.PathValue("connectionID")); err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

func (s *Server) handleListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	list, err := s.scm.ListDeliveries(r.Context(), project.ID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = []scm.Delivery{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"deliveries": list})
}

func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	unread := r.URL.Query().Get("unread") == "1"
	list, err := s.notifications.ListForUser(r.Context(), currentUser(r).ID, unread, 40)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	count, _ := s.notifications.UnreadCount(r.Context(), currentUser(r).ID)
	if list == nil {
		list = []notifications.Notification{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": list, "unread_count": count})
}

func (s *Server) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	if err := s.notifications.MarkRead(r.Context(), currentUser(r).ID, r.PathValue("notificationID")); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "read"})
}

func (s *Server) handleMarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	if err := s.notifications.MarkAllRead(r.Context(), currentUser(r).ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "read"})
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	provider := scm.NormalizeProvider(r.PathValue("provider"))
	connectionID := r.URL.Query().Get("connection_id")
	projectID := r.URL.Query().Get("project_id")

	conn, connErr := s.scm.FindForWebhook(r.Context(), provider, projectID, connectionID)
	secret := ""
	if connErr == nil {
		secret = conn.WebhookSecret
		provider = conn.Provider
		projectID = conn.ProjectID
	}
	if secret == "" {
		secret = s.opts.WebhookSecret
	}
	if secret == "" {
		secret = os.Getenv("SHIPYARD_WEBHOOK_SECRET")
	}
	if secret != "" && !scm.VerifyRequest(provider, secret, r.Header, body) {
		writeError(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	eventType := scm.EventTypeFromHeaders(provider, r.Header)
	deliveryID := scm.DeliveryIDFromHeaders(r.Header)
	parsed := scm.ParseEvent(provider, eventType, body)

	summary := parsed.EventType
	if parsed.Title != "" {
		summary = parsed.Title
	}
	status := "accepted"
	runID := ""
	errMsg := ""

	if connErr != nil {
		status = "ignored"
		errMsg = "no matching scm connection"
		_, _ = s.scm.RecordDelivery(r.Context(), scm.DeliveryInput{
			Provider: provider, EventType: parsed.EventType, DeliveryID: deliveryID,
			Status: status, ErrorMessage: errMsg, Summary: summary, Payload: json.RawMessage(body),
			ProjectID: projectID,
		})
		writeJSON(w, http.StatusAccepted, map[string]any{"status": status, "reason": errMsg})
		return
	}

	if !parsed.ShouldBuild {
		status = "ignored"
		errMsg = parsed.IgnoreReason
		if errMsg == "" {
			errMsg = "event does not trigger a build"
		}
	} else {
		pipelineID, perr := s.resolvePipelineID(r, conn)
		if perr != nil {
			status = "failed"
			errMsg = perr.Error()
		} else {
			actor := ""
			if conn.CreatedBy != "" {
				actor = conn.CreatedBy
			}
			// StartRun requires actor UUID; fall back to first org member if connection has none.
			if actor == "" {
				members, _ := s.orgs.ListMembers(r.Context(), conn.OrganizationID)
				if len(members) > 0 {
					actor = members[0].UserID
				}
			}
			if actor == "" {
				status = "failed"
				errMsg = "no actor for webhook run"
			} else {
				run, serr := s.pipelines.StartRun(r.Context(), conn.OrganizationID, conn.ProjectID, pipelineID, actor, "webhook", parsed.GitRef, parsed.GitSHA)
				if serr != nil {
					status = "failed"
					errMsg = serr.Error()
				} else {
					status = "processed"
					runID = run.ID
					_ = s.pipelines.AdvanceRunGraph(r.Context(), run.ID)
					public := strings.TrimRight(s.opts.PublicURL, "/")
					runURL := fmt.Sprintf("%s/pipelines/runs/%s", public, run.ID)
					_ = s.scm.AttachRunSCM(r.Context(), run.ID, conn.ID, parsed.PRNumber, "", runURL)
					if parsed.PRNumber > 0 {
						bodyComment := scm.FormatStartedComment(conn.BotUsername, runURL, run.Number, parsed.GitRef, parsed.GitSHA)
						if res, cerr := scm.PostStatusComment(r.Context(), scm.CommentInput{
							Connection: conn, PRNumber: parsed.PRNumber, GitSHA: parsed.GitSHA, Body: bodyComment,
						}); cerr == nil && res.ID != "" {
							_ = s.scm.AttachRunSCM(r.Context(), run.ID, conn.ID, parsed.PRNumber, res.ID, runURL)
						}
					}
					_ = s.notifications.NotifyOrgMembers(r.Context(), conn.OrganizationID, conn.ProjectID,
						"run.started",
						fmt.Sprintf("Build started · #%d", run.Number),
						fmt.Sprintf("%s %s triggered a run on %s", conn.Provider, parsed.EventType, parsed.GitRef),
						"/pipelines/runs/"+run.ID,
					)
				}
			}
		}
	}

	d, _ := s.scm.RecordDelivery(r.Context(), scm.DeliveryInput{
		ConnectionID: conn.ID, OrganizationID: conn.OrganizationID, ProjectID: conn.ProjectID,
		Provider: provider, EventType: parsed.EventType, DeliveryID: deliveryID,
		Status: status, RunID: runID, ErrorMessage: errMsg, Summary: summary, Payload: json.RawMessage(body),
	})
	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":      status,
		"delivery_id": d.ID,
		"run_id":      runID,
		"error":       errMsg,
	})
}

func (s *Server) resolvePipelineID(r *http.Request, conn scm.Connection) (string, error) {
	defs, err := s.pipelines.ListDefinitions(r.Context(), conn.ProjectID)
	if err != nil {
		return "", err
	}
	if len(defs) == 0 {
		return "", fmt.Errorf("%w: no pipelines in project", identity.ErrNotFound)
	}
	if conn.PipelineSlug != "" {
		for _, d := range defs {
			if d.Slug == conn.PipelineSlug {
				return d.ID, nil
			}
		}
		return "", fmt.Errorf("%w: pipeline slug not found", identity.ErrNotFound)
	}
	return defs[0].ID, nil
}

func (s *Server) maybeNotifyRunFinished(runID string) {
	ctx := context.Background()
	info, err := s.scm.GetRunSCM(ctx, runID)
	if err != nil {
		return
	}
	if info.Status != "succeeded" && info.Status != "failed" && info.Status != "canceled" {
		return
	}
	public := strings.TrimRight(s.opts.PublicURL, "/")
	runURL := info.TargetURL
	if runURL == "" {
		runURL = fmt.Sprintf("%s/pipelines/runs/%s", public, info.RunID)
	}
	_ = s.notifications.NotifyOrgMembers(ctx, info.OrganizationID, info.ProjectID,
		"run."+info.Status,
		fmt.Sprintf("Run #%d %s", info.Number, info.Status),
		fmt.Sprintf("Pipeline finished with status %s", info.Status),
		"/pipelines/runs/"+info.RunID,
	)
	if info.ConnectionID == "" || info.PRNumber <= 0 {
		return
	}
	conn, err := s.scm.GetByID(ctx, info.ConnectionID)
	if err != nil {
		return
	}
	body := scm.FormatFinishedComment(conn.BotUsername, runURL, info.Number, info.Status, info.GitRef)
	_, _ = scm.PostStatusComment(ctx, scm.CommentInput{
		Connection: conn, PRNumber: info.PRNumber, GitSHA: info.GitSHA, Body: body,
	})
}
