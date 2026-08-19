package httpapi

import (
	"context"
	"net/http"
	"time"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/pipeline"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/runners"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/scm"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/secrets"
)

type runnerRegisterRequest struct {
	Token        string   `json:"token"`
	Name         string   `json:"name"`
	Labels       []string `json:"labels"`
	Capabilities []string `json:"capabilities"`
}

func (s *Server) handleRunnerRegister(w http.ResponseWriter, r *http.Request) {
	var req runnerRegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	runner, token, err := s.runners.Register(r.Context(), req.Token, req.Name, req.Labels, req.Capabilities)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"runner": runner, "runner_token": token})
}

func (s *Server) handleDeleteRunner(w http.ResponseWriter, r *http.Request) {
	if err := s.runners.Delete(r.Context(), r.PathValue("runnerID")); err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

func (s *Server) handleRunnerHeartbeat(w http.ResponseWriter, r *http.Request) {
	if err := s.runners.Heartbeat(r.Context(), currentRunner(r).ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleRunnerLease(w http.ResponseWriter, r *http.Request) {
	job, err := s.runners.LeaseNextJob(r.Context(), currentRunner(r), 2*time.Minute)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if job == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.fillLeaseContext(r, job)
	writeJSON(w, http.StatusOK, map[string]any{"job": job})
}

func (s *Server) fillLeaseContext(r *http.Request, job *runners.LeaseJob) {
	ctx := r.Context()
	var run runners.LeaseRun
	var orgID, projectID, pipelineID, connectionID string
	if err := s.pool.QueryRow(ctx, `
		SELECT r.organization_id, r.project_id, r.number, r.git_ref, r.git_sha, r.pipeline_id, COALESCE(r.scm_connection_id::text,''), p.slug, o.slug
		FROM pipeline_runs r
		JOIN projects p ON p.id = r.project_id
		JOIN organizations o ON o.id = r.organization_id
		WHERE r.id = $1
	`, job.RunID).Scan(&orgID, &projectID, &run.Number, &run.GitRef, &run.GitSHA, &pipelineID, &connectionID, &run.ProjectSlug, &run.OrgSlug); err != nil {
		return
	}
	run.ID = job.RunID
	job.Run = &run

	if s.secrets != nil {
		if named, err := s.secrets.NamedValuesForScope(ctx, orgID, projectID); err == nil && len(named) > 0 {
			job.Secrets = map[string]string{}
			for name, value := range named {
				job.Secrets[secrets.EnvName(name)] = value
			}
		}
	}

	if def, err := s.pipelines.GetDefinition(ctx, projectID, pipelineID); err == nil {
		if doc, err := pipeline.Parse(def.YAML); err == nil {
			if env := doc.Jobs[job.Name].Env; len(env) > 0 {
				job.Env = env
			}
		}
	}

	conn, ok := s.leaseConnection(ctx, projectID, connectionID)
	if !ok {
		return
	}
	cloneURL := scm.CloneURL(conn)
	if cloneURL == "" {
		return
	}
	job.Repo = &runners.LeaseRepo{
		CloneURL: cloneURL,
		Username: scm.GitUsername(conn.Provider),
		Token:    conn.AccessToken,
	}
}

func (s *Server) leaseConnection(ctx context.Context, projectID, connectionID string) (scm.Connection, bool) {
	if connectionID != "" {
		conn, err := s.scm.GetByID(ctx, connectionID)
		if err != nil {
			return scm.Connection{}, false
		}
		return conn, true
	}
	list, err := s.scm.List(ctx, projectID)
	if err != nil {
		return scm.Connection{}, false
	}
	for _, c := range list {
		if !c.Enabled {
			continue
		}
		conn, err := s.scm.Get(ctx, projectID, c.ID)
		if err != nil {
			return scm.Connection{}, false
		}
		return conn, true
	}
	return scm.Connection{}, false
}

type leaseBody struct {
	LeaseID string `json:"lease_id"`
}

func (s *Server) handleRunnerStartJob(w http.ResponseWriter, r *http.Request) {
	var req leaseBody
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.runners.MarkJobRunning(r.Context(), r.PathValue("jobID"), req.LeaseID); err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "running"})
}

func (s *Server) handleRunnerRenewLease(w http.ResponseWriter, r *http.Request) {
	var req leaseBody
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	status, err := s.runners.RenewLease(r.Context(), currentRunner(r).ID, r.PathValue("jobID"), req.LeaseID, 2*time.Minute)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": status})
}

type completeJobRequest struct {
	LeaseID string `json:"lease_id"`
	Status  string `json:"status"`
	Error   string `json:"error"`
}

func (s *Server) handleRunnerCompleteJob(w http.ResponseWriter, r *http.Request) {
	var req completeJobRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	runID, err := s.runners.CompleteJob(r.Context(), r.PathValue("jobID"), req.LeaseID, req.Status, req.Error)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	_ = s.pipelines.AdvanceRunGraph(r.Context(), runID)
	go s.maybeNotifyRunFinished(runID)
	writeJSON(w, http.StatusOK, map[string]any{"status": req.Status})
}

type appendLogRequest struct {
	LeaseID string `json:"lease_id"`
	StepID  string `json:"step_id"`
	Stream  string `json:"stream"`
	Line    string `json:"line"`
}

func (s *Server) handleRunnerAppendLog(w http.ResponseWriter, r *http.Request) {
	var req appendLogRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.runners.AppendLog(r.Context(), currentRunner(r).ID, r.PathValue("jobID"), req.StepID, req.Stream, req.Line); err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "ok"})
}

type stepStatusRequest struct {
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code"`
}

func (s *Server) handleRunnerStepStatus(w http.ResponseWriter, r *http.Request) {
	var req stepStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.runners.UpdateStepStatus(r.Context(), currentRunner(r).ID, r.PathValue("stepID"), req.Status, req.ExitCode); err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": req.Status})
}

func (s *Server) handleRunnerJobSteps(w http.ResponseWriter, r *http.Request) {
	steps, err := s.runners.GetJobSteps(r.Context(), currentRunner(r).ID, r.PathValue("jobID"))
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	if steps == nil {
		steps = []runners.Step{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"steps": steps})
}
