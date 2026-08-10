package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/rbac"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/scm"
)

type forgeCredentialRequest struct {
	Provider    string `json:"provider"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	BaseURL     string `json:"base_url"`
	AccessToken string `json:"access_token"`
}

type forgeImportRequest struct {
	CredentialID string          `json:"credential_id"`
	RemoteOrg    string          `json:"remote_org"`
	Repos        []scm.RemoteRepo `json:"repos"`
}

func (s *Server) handleListForgeCredentials(w http.ResponseWriter, r *http.Request) {
	org, ok := s.orgAccess(w, r, rbac.PermOrgRead)
	if !ok {
		return
	}
	list, err := s.scm.ListCredentials(r.Context(), org.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = []scm.ForgeCredential{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"credentials": list})
}

func (s *Server) handleCreateForgeCredential(w http.ResponseWriter, r *http.Request) {
	org, ok := s.orgAccess(w, r, rbac.PermOrgUpdate)
	if !ok {
		return
	}
	var req forgeCredentialRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	cred, err := s.scm.CreateCredential(r.Context(), scm.CreateCredentialInput{
		OrganizationID: org.ID,
		Provider:       req.Provider,
		Kind:           req.Kind,
		Name:           req.Name,
		BaseURL:        req.BaseURL,
		AccessToken:    req.AccessToken,
		ActorID:        currentUser(r).ID,
	})
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"credential": cred})
}

func (s *Server) handleDeleteForgeCredential(w http.ResponseWriter, r *http.Request) {
	org, ok := s.orgAccess(w, r, rbac.PermOrgUpdate)
	if !ok {
		return
	}
	if err := s.scm.DeleteCredential(r.Context(), org.ID, r.PathValue("credentialID")); err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

func (s *Server) handleListRemoteOrgs(w http.ResponseWriter, r *http.Request) {
	org, ok := s.orgAccess(w, r, rbac.PermOrgRead)
	if !ok {
		return
	}
	cred, err := s.scm.GetCredential(r.Context(), org.ID, r.PathValue("credentialID"))
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	list, err := s.scm.ListRemoteOrgs(r.Context(), cred)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if list == nil {
		list = []scm.RemoteOrg{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"orgs": list})
}

func (s *Server) handleListRemoteRepos(w http.ResponseWriter, r *http.Request) {
	org, ok := s.orgAccess(w, r, rbac.PermOrgRead)
	if !ok {
		return
	}
	cred, err := s.scm.GetCredential(r.Context(), org.ID, r.PathValue("credentialID"))
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	includeArchived := q.Get("include_archived") == "true" || q.Get("include_archived") == "1"
	list, err := s.scm.ListRemoteRepos(r.Context(), cred, q.Get("org"), q.Get("q"), page, limit, includeArchived)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if list == nil {
		list = []scm.RemoteRepo{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"repos": list})
}

func (s *Server) handleStartForgeImport(w http.ResponseWriter, r *http.Request) {
	org, ok := s.orgAccess(w, r, rbac.PermOrgUpdate)
	if !ok {
		return
	}
	var req forgeImportRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	public := strings.TrimSpace(s.opts.APIURL)
	if public == "" {
		public = strings.TrimSpace(s.opts.PublicURL)
	}
	job, err := s.scm.StartImport(r.Context(), scm.StartImportInput{
		OrganizationID: org.ID,
		CredentialID:   req.CredentialID,
		RemoteOrg:      req.RemoteOrg,
		Repos:          req.Repos,
		PublicBaseURL:  public,
		ActorID:        currentUser(r).ID,
		Orgs:           s.orgs,
	})
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job})
}

func (s *Server) handleGetForgeImport(w http.ResponseWriter, r *http.Request) {
	org, ok := s.orgAccess(w, r, rbac.PermOrgRead)
	if !ok {
		return
	}
	job, items, err := s.scm.GetImportJob(r.Context(), org.ID, r.PathValue("jobID"))
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	if items == nil {
		items = []scm.ImportJobItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job, "items": items})
}
