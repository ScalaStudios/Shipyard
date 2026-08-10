package httpapi

import (
	"net/http"
	"time"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/runners"
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
	writeJSON(w, http.StatusOK, map[string]any{"job": job})
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
	line := req.Line
	if s.secrets != nil {
		var orgID, projectID string
		err := s.pool.QueryRow(r.Context(), `
			SELECT p.organization_id, pr.project_id
			FROM jobs j
			JOIN pipeline_runs pr ON pr.id = j.run_id
			JOIN projects p ON p.id = pr.project_id
			WHERE j.id = $1
		`, r.PathValue("jobID")).Scan(&orgID, &projectID)
		if err == nil {
			if values, err := s.secrets.ValuesForScope(r.Context(), orgID, projectID); err == nil {
				line = secrets.MaskLine(line, values)
			}
		}
	}
	if err := s.runners.AppendLog(r.Context(), r.PathValue("jobID"), req.StepID, req.Stream, line); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
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
	if err := s.runners.UpdateStepStatus(r.Context(), r.PathValue("stepID"), req.Status, req.ExitCode); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": req.Status})
}

func (s *Server) handleRunnerJobSteps(w http.ResponseWriter, r *http.Request) {
	steps, err := s.runners.GetJobSteps(r.Context(), r.PathValue("jobID"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if steps == nil {
		steps = []runners.Step{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"steps": steps})
}
