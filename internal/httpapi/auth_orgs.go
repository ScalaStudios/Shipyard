package httpapi

import (
	"net/http"
	"time"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/audit"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/orgs"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/rbac"
)

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "error": "database unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	count, err := s.identity.UserCount(r.Context())
	allowRegister := s.allowRegister(r.Context())
	if err == nil && count == 0 {
		allowRegister = true
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"product":        "shipyard",
		"phase":          "hardening",
		"started_at":     s.started.Format(time.RFC3339),
		"allow_register": allowRegister,
		"node_id":        s.cluster.NodeID(),
		"oidc":           s.oidc != nil && s.oidc.Enabled(),
		"secrets":        s.secrets != nil,
	})
}

type credentialsRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	Login       string `json:"login"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	count, err := s.identity.UserCount(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if count > 0 && !s.allowRegister(r.Context()) {
		writeError(w, http.StatusForbidden, "registration disabled")
		return
	}
	user, err := s.identity.CreateUser(r.Context(), req.Username, req.Email, req.DisplayName, req.Password)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	token, expires, err := s.identity.CreateSession(r.Context(), user.ID, s.opts.SessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.setSessionCookie(w, r, token, expires)
	_ = s.audit.Record(r.Context(), audit.Event{ActorUserID: &user.ID, Action: "user.registered", ResourceType: "user", ResourceID: user.ID, IP: clientIP(r), UserAgent: r.UserAgent()})
	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	login := req.Login
	if login == "" {
		login = req.Username
	}
	if login == "" {
		login = req.Email
	}
	user, err := s.identity.Authenticate(r.Context(), login, req.Password)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	token, expires, err := s.identity.CreateSession(r.Context(), user.ID, s.opts.SessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.setSessionCookie(w, r, token, expires)
	_ = s.audit.Record(r.Context(), audit.Event{ActorUserID: &user.ID, Action: "user.login", ResourceType: "user", ResourceID: user.ID, IP: clientIP(r), UserAgent: r.UserAgent()})
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		_ = s.identity.RevokeSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"user": currentUser(r)})
}

type createTokenRequest struct {
	Name string `json:"name"`
	TTL  string `json:"ttl"`
}

func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req createTokenRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	user := currentUser(r)
	var ttl *time.Duration
	if req.TTL != "" {
		d, err := time.ParseDuration(req.TTL)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid ttl")
			return
		}
		ttl = &d
	}
	plain, prefix, expires, err := s.identity.CreateAPIToken(r.Context(), user.ID, req.Name, ttl)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	_ = s.audit.Record(r.Context(), audit.Event{ActorUserID: &user.ID, Action: "token.created", ResourceType: "api_token", ResourceID: prefix, IP: clientIP(r), UserAgent: r.UserAgent(), Metadata: map[string]any{"name": req.Name}})
	writeJSON(w, http.StatusCreated, map[string]any{"token": plain, "prefix": prefix, "expires_at": expires})
}

func (s *Server) handleListOrgs(w http.ResponseWriter, r *http.Request) {
	list, err := s.orgs.ListOrganizations(r.Context(), currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = []orgs.Organization{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"organizations": list})
}

type orgRequest struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	var req orgRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	user := currentUser(r)
	org, err := s.orgs.CreateOrganization(r.Context(), user.ID, req.Slug, req.Name, req.Description)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	_ = s.audit.Record(r.Context(), audit.Event{ActorUserID: &user.ID, Action: "organization.created", ResourceType: "organization", ResourceID: org.ID, OrganizationID: &org.ID, IP: clientIP(r), UserAgent: r.UserAgent(), Metadata: map[string]any{"slug": org.Slug}})
	writeJSON(w, http.StatusCreated, map[string]any{"organization": org})
}

func (s *Server) handleGetOrg(w http.ResponseWriter, r *http.Request) {
	org, _, err := s.orgs.Require(r.Context(), currentUser(r).ID, r.PathValue("orgID"), rbac.PermOrgRead)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"organization": org})
}

func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	org, ok := s.orgAccess(w, r, rbac.PermOrgRead)
	if !ok {
		return
	}
	members, err := s.orgs.ListMembers(r.Context(), org.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if members == nil {
		members = []orgs.Member{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

type memberRequest struct {
	Login string `json:"login"`
	Role  string `json:"role"`
}

func (s *Server) handleAddMember(w http.ResponseWriter, r *http.Request) {
	org, ok := s.orgAccess(w, r, rbac.PermOrgManageMembers)
	if !ok {
		return
	}
	orgID := org.ID
	actor := currentUser(r)
	var req memberRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	role, ok := rbac.ParseRole(req.Role)
	if !ok {
		role = rbac.RoleDeveloper
	}
	user, err := s.identity.FindUser(r.Context(), req.Login)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	if err := s.orgs.AddMember(r.Context(), orgID, user.ID, role); err != nil {
		mapIdentityError(w, err)
		return
	}
	_ = s.audit.Record(r.Context(), audit.Event{
		ActorUserID:    &actor.ID,
		Action:         "org.member_added",
		ResourceType:   "organization",
		ResourceID:     orgID,
		OrganizationID: &orgID,
		IP:             clientIP(r),
		UserAgent:      r.UserAgent(),
		Metadata:       map[string]any{"user_id": user.ID, "role": role},
	})
	writeJSON(w, http.StatusCreated, map[string]any{"status": "ok", "user_id": user.ID, "role": role})
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	org, ok := s.orgAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	list, err := s.orgs.ListProjects(r.Context(), org.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = []orgs.Project{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": list})
}

type projectRequest struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	org, ok := s.orgAccess(w, r, rbac.PermProjectCreate)
	if !ok {
		return
	}
	orgID := org.ID
	user := currentUser(r)
	var req projectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	project, err := s.orgs.CreateProject(r.Context(), user.ID, orgID, req.Slug, req.Name, req.Description)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	_ = s.audit.Record(r.Context(), audit.Event{ActorUserID: &user.ID, Action: "project.created", ResourceType: "project", ResourceID: project.ID, OrganizationID: &orgID, ProjectID: &project.ID, IP: clientIP(r), UserAgent: r.UserAgent(), Metadata: map[string]any{"slug": project.Slug}})
	writeJSON(w, http.StatusCreated, map[string]any{"project": project})
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	org, ok := s.orgAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	project, err := s.orgs.GetProject(r.Context(), org.ID, r.PathValue("projectID"))
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": project})
}
