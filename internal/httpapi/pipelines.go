package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/orgs"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/pipeline"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/rbac"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/runners"
)

type pipelineRequest struct {
	Slug string `json:"slug"`
	YAML string `json:"yaml"`
}

func (s *Server) handleListPipelines(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	list, err := s.pipelines.ListDefinitions(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = []pipeline.Definition{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"pipelines": list})
}

func (s *Server) handleUpsertPipeline(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectUpdate)
	if !ok {
		return
	}
	var req pipelineRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	def, err := s.pipelines.UpsertDefinition(r.Context(), project.ID, currentUser(r).ID, req.Slug, req.YAML)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pipeline": def})
}

type startRunRequest struct {
	GitRef string `json:"git_ref"`
	GitSHA string `json:"git_sha"`
}

func (s *Server) handleStartRun(w http.ResponseWriter, r *http.Request) {
	org, project, ok := s.projectAccess(w, r, rbac.PermProjectUpdate)
	if !ok {
		return
	}
	var req startRunRequest
	_ = decodeJSON(r, &req)
	run, err := s.pipelines.StartRun(r.Context(), org.ID, project.ID, r.PathValue("pipelineID"), currentUser(r).ID, "manual", req.GitRef, req.GitSHA)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	_ = s.pipelines.AdvanceRunGraph(r.Context(), run.ID)
	pipelineSlugOrID := r.PathValue("pipelineID")
	if def, derr := s.pipelines.GetDefinition(r.Context(), project.ID, run.PipelineID); derr == nil && def.Slug != "" {
		pipelineSlugOrID = def.Slug
	}
	s.fanoutNotify(r.Context(), org.ID, project.ID, "run.started",
		fmt.Sprintf("Build started · #%d", run.Number),
		fmt.Sprintf("%s started a run of %s", currentUser(r).Username, pipelineSlugOrID),
		"/pipelines/runs/"+run.ID,
		run.Number, "started", run.GitRef, run.GitSHA, project.Slug,
	)
	writeJSON(w, http.StatusCreated, map[string]any{"run": run})
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	list, err := s.pipelines.ListRuns(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = []pipeline.Run{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": list})
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	run, err := s.pipelines.GetRun(r.Context(), project.ID, r.PathValue("runID"))
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	jobs, err := s.pipelines.ListJobs(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if jobs == nil {
		jobs = []pipeline.Job{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run, "jobs": jobs})
}

func (s *Server) handleCancelRun(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectUpdate)
	if !ok {
		return
	}
	if err := s.pipelines.CancelRun(r.Context(), project.ID, r.PathValue("runID")); err != nil {
		mapIdentityError(w, err)
		return
	}
	go s.maybeNotifyRunFinished(r.PathValue("runID"))
	writeJSON(w, http.StatusOK, map[string]any{"status": "canceled"})
}

func (s *Server) handleListRunJobs(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	run, err := s.pipelines.GetRun(r.Context(), project.ID, r.PathValue("runID"))
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	jobs, err := s.pipelines.ListJobs(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if jobs == nil {
		jobs = []pipeline.Job{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (s *Server) handleJobLogs(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	logs, err := s.runners.ListLogs(r.Context(), project.ID, r.PathValue("jobID"), queryInt64(r, "after", 0))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if logs == nil {
		logs = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
}

func (s *Server) handleListRunners(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var list []runners.Runner
	var err error
	if user.IsAdmin {
		list, err = s.runners.List(r.Context())
	} else {
		var orgIDs []string
		var memberships []orgs.Organization
		memberships, err = s.orgs.ListOrganizations(r.Context(), user.ID)
		if err == nil {
			for _, org := range memberships {
				orgIDs = append(orgIDs, org.ID)
			}
			list, err = s.runners.ListForOrgs(r.Context(), orgIDs)
		}
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runners": list})
}

type regTokenRequest struct {
	OrganizationID string `json:"organization_id"`
	TTL            string `json:"ttl"`
}

func (s *Server) handleCreateRunnerRegToken(w http.ResponseWriter, r *http.Request) {
	var req regTokenRequest
	_ = decodeJSON(r, &req)
	ttl := 1 * time.Hour
	if req.TTL != "" {
		if d, err := time.ParseDuration(req.TTL); err == nil {
			ttl = d
		}
	}
	if !s.requireRunnerScope(w, r, req.OrganizationID) {
		return
	}
	token, expires, err := s.runners.CreateRegistrationToken(r.Context(), req.OrganizationID, currentUser(r).ID, ttl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":      token,
		"expires_at": expires,
		"api_url":    s.apiBaseURL(r),
		"curl_command": fmt.Sprintf(
			"curl -fsSL %q | bash",
			s.apiBaseURL(r)+"/api/v1/runners/install.sh?token="+url.QueryEscape(token)+"&name=runner-1&labels=linux&url="+url.QueryEscape(s.apiBaseURL(r)),
		),
	})
}
