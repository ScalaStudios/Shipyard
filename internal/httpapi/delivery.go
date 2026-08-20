package httpapi

import (
	"io"
	"mime"
	"net/http"
	"strconv"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/artifacts"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/packages"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/rbac"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/releases"
)

func (s *Server) handleListArtifacts(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	list, err := s.artifacts.List(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = []artifacts.Artifact{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"artifacts": list})
}

func (s *Server) handleUploadArtifact(w http.ResponseWriter, r *http.Request) {
	org, project, ok := s.projectAccess(w, r, rbac.PermProjectUpdate)
	if !ok {
		return
	}
	name := r.URL.Query().Get("name")
	contentType := r.Header.Get("Content-Type")
	size, err := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		size = -1
	}
	a, err := s.artifacts.Put(r.Context(), org.ID, project.ID, currentUser(r).ID, name, contentType, r.URL.Query().Get("job_id"), r.URL.Query().Get("run_id"), r.Body, size)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"artifact": a})
}

func (s *Server) handleDownloadArtifact(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	rc, a, err := s.artifacts.Open(r.Context(), project.ID, r.PathValue("artifactID"))
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", a.ContentType)
	w.Header().Set("Digest", a.Digest)
	w.Header().Set("Content-Length", strconv.FormatInt(a.SizeBytes, 10))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": a.Name}))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}

type envRequest struct {
	Slug               string `json:"slug"`
	Name               string `json:"name"`
	DeployPipelineSlug string `json:"deploy_pipeline_slug"`
}

func (s *Server) handleListEnvironments(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	list, err := s.releases.ListEnvironments(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = []releases.Environment{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"environments": list})
}

func (s *Server) handleCreateEnvironment(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectUpdate)
	if !ok {
		return
	}
	var req envRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	env, err := s.releases.CreateEnvironment(r.Context(), project.ID, req.Slug, req.Name, req.DeployPipelineSlug)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"environment": env})
}

type releaseRequest struct {
	Version     string   `json:"version"`
	Title       string   `json:"title"`
	Notes       string   `json:"notes"`
	RunID       string   `json:"run_id"`
	ArtifactIDs []string `json:"artifact_ids"`
}

func (s *Server) handleListReleases(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	list, err := s.releases.ListReleases(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = []releases.Release{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"releases": list})
}

func (s *Server) handleCreateRelease(w http.ResponseWriter, r *http.Request) {
	org, project, ok := s.projectAccess(w, r, rbac.PermProjectUpdate)
	if !ok {
		return
	}
	var req releaseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	rel, err := s.releases.CreateRelease(r.Context(), org.ID, project.ID, currentUser(r).ID, req.Version, req.Title, req.Notes, req.RunID, req.ArtifactIDs)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"release": rel})
}

type deploymentRequest struct {
	EnvironmentID string `json:"environment_id"`
	ReleaseID     string `json:"release_id"`
}

func (s *Server) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	list, err := s.releases.ListDeployments(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = []releases.Deployment{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"deployments": list})
}

func (s *Server) handleCreateDeployment(w http.ResponseWriter, r *http.Request) {
	org, project, ok := s.projectAccess(w, r, rbac.PermProjectUpdate)
	if !ok {
		return
	}
	var req deploymentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	dep, slug, err := s.releases.CreateDeployment(r.Context(), project.ID, req.EnvironmentID, req.ReleaseID, currentUser(r).ID)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	if slug == "" {
		writeJSON(w, http.StatusCreated, map[string]any{"deployment": dep})
		return
	}
	def, err := s.pipelines.GetDefinitionBySlug(r.Context(), project.ID, slug)
	if err != nil {
		writeError(w, http.StatusBadRequest, "environment deploy pipeline not found")
		return
	}
	run, err := s.pipelines.StartRun(r.Context(), org.ID, project.ID, def.ID, currentUser(r).ID, "deployment", "", "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	_ = s.pipelines.AdvanceRunGraph(r.Context(), run.ID)
	if err := s.releases.AttachDeploymentRun(r.Context(), dep.ID, run.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	dep.RunID = run.ID
	dep.Status = "running"
	writeJSON(w, http.StatusCreated, map[string]any{"deployment": dep})
}

type packageRepoRequest struct {
	Name   string `json:"name"`
	Format string `json:"format"`
}

func (s *Server) handleListPackageRepos(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	list, err := s.packages.ListRepositories(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = []packages.Repository{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": list})
}

func (s *Server) handleCreatePackageRepo(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectUpdate)
	if !ok {
		return
	}
	var req packageRepoRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	repo, err := s.packages.CreateRepository(r.Context(), project.ID, req.Name, req.Format)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"repository": repo})
}

func (s *Server) handleListPackageVersions(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	repo, err := s.packages.GetRepository(r.Context(), project.ID, r.PathValue("repoID"))
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	list, err := s.packages.ListVersions(r.Context(), repo.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = []packages.Version{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": list})
}

func (s *Server) handlePublishPackage(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectUpdate)
	if !ok {
		return
	}
	repo, err := s.packages.GetRepository(r.Context(), project.ID, r.PathValue("repoID"))
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	name := r.URL.Query().Get("name")
	version := r.URL.Query().Get("version")
	size, err := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		size = -1
	}
	v, err := s.packages.Publish(r.Context(), repo.ID, name, version, "", r.Body, size, nil)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"version": v})
}

func (s *Server) handleListOCIRepos(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	list, err := s.registry.ListRepositories(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": list})
}

func (s *Server) handlePutOCIManifest(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectUpdate)
	if !ok {
		return
	}
	repo, err := s.registry.EnsureRepository(r.Context(), project.ID, r.PathValue("name"))
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	raw, err := readBodyLimited(r, 8<<20)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	m, err := s.registry.PutManifest(r.Context(), repo.ID, r.Header.Get("Content-Type"), r.PathValue("tag"), raw)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"manifest": m})
}

func (s *Server) handleListOCITags(w http.ResponseWriter, r *http.Request) {
	_, project, ok := s.projectAccess(w, r, rbac.PermProjectRead)
	if !ok {
		return
	}
	repo, err := s.registry.EnsureRepository(r.Context(), project.ID, r.PathValue("name"))
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	tags, err := s.registry.ListTags(r.Context(), repo.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if tags == nil {
		tags = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": tags})
}
