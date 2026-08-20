package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/rbac"
)

func writeOCIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"errors": []map[string]any{{"code": code, "message": message}},
	})
}

func mapOCIError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, identity.ErrInvalidInput):
		writeOCIError(w, http.StatusBadRequest, "DIGEST_INVALID", err.Error())
	case errors.Is(err, identity.ErrNotFound):
		writeOCIError(w, http.StatusNotFound, "BLOB_UPLOAD_UNKNOWN", "upload unknown")
	default:
		writeOCIError(w, http.StatusInternalServerError, "UNSUPPORTED", "internal error")
	}
}

func (s *Server) handleOCIAPIVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (s *Server) handleOCIGet(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(r.PathValue("rest"), "/")
	if rest == "" {
		s.requireRegistryAuth(s.handleOCIAPIVersion)(w, r)
		return
	}
	s.requireRegistryAuth(s.handleOCIDistribution)(w, r)
}

func (s *Server) handleOCIDistribution(w http.ResponseWriter, r *http.Request) {
	kind, name, a := parseOCIRest(r.PathValue("rest"))
	switch kind {
	case "blob":
		switch r.Method {
		case http.MethodHead:
			s.ociBlobStat(w, r, name, a)
			return
		case http.MethodGet:
			s.ociBlobGet(w, r, name, a)
			return
		}
	case "upload-start":
		if r.Method == http.MethodPost {
			s.ociBlobUploadStart(w, r, name)
			return
		}
	case "upload-complete":
		switch r.Method {
		case http.MethodPatch:
			s.ociBlobUploadPatch(w, r, name, a)
			return
		case http.MethodPut:
			s.ociBlobUploadComplete(w, r, name, a)
			return
		}
	case "manifest":
		switch r.Method {
		case http.MethodPut:
			s.ociManifestPut(w, r, name, a)
			return
		case http.MethodGet, http.MethodHead:
			s.ociManifestGet(w, r, name, a)
			return
		}
	case "tags":
		if r.Method == http.MethodGet {
			s.ociTagsList(w, r, name)
			return
		}
	}
	writeOCIError(w, http.StatusNotFound, "UNSUPPORTED", "unsupported request")
}

func parseOCIRest(rest string) (kind, name, arg string) {
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return "", "", ""
	}
	if idx := strings.Index(rest, "/blobs/uploads/"); idx >= 0 {
		name = rest[:idx]
		rem := strings.Trim(rest[idx+len("/blobs/uploads/"):], "/")
		if rem == "" {
			return "upload-start", name, ""
		}
		return "upload-complete", name, rem
	}
	if strings.HasSuffix(rest, "/blobs/uploads") {
		return "upload-start", strings.TrimSuffix(rest, "/blobs/uploads"), ""
	}
	if strings.HasSuffix(rest, "/tags/list") {
		return "tags", strings.TrimSuffix(rest, "/tags/list"), ""
	}
	if idx := strings.Index(rest, "/blobs/"); idx >= 0 {
		return "blob", rest[:idx], rest[idx+len("/blobs/"):]
	}
	if idx := strings.Index(rest, "/manifests/"); idx >= 0 {
		return "manifest", rest[:idx], rest[idx+len("/manifests/"):]
	}
	return "", "", ""
}

func (s *Server) ociRepo(w http.ResponseWriter, r *http.Request, name string, perm rbac.Permission) (projectID, repoID string, ok bool) {
	if !registryScopeOK(r, perm) {
		writeOCIError(w, http.StatusForbidden, "DENIED", "token missing registry scope")
		return "", "", false
	}
	parts := strings.SplitN(name, "/", 3)
	if len(parts) < 3 {
		writeOCIError(w, http.StatusBadRequest, "NAME_INVALID", "repository name must be org/project/name")
		return "", "", false
	}
	_, project, err := s.orgs.RequireBySlug(r.Context(), currentUser(r).ID, parts[0], parts[1], perm)
	if err != nil {
		switch {
		case errors.Is(err, identity.ErrNotFound):
			writeOCIError(w, http.StatusNotFound, "NAME_UNKNOWN", "repository not found")
		case errors.Is(err, identity.ErrForbidden), errors.Is(err, identity.ErrUnauthorized):
			writeOCIError(w, http.StatusForbidden, "DENIED", "access denied")
		default:
			writeOCIError(w, http.StatusInternalServerError, "UNSUPPORTED", "internal error")
		}
		return "", "", false
	}
	if perm == rbac.PermProjectRead {
		repo, err := s.registry.GetRepository(r.Context(), project.ID, parts[2])
		if err != nil {
			writeOCIError(w, http.StatusNotFound, "NAME_UNKNOWN", "repository not found")
			return "", "", false
		}
		return project.ID, repo.ID, true
	}
	repo, err := s.registry.EnsureRepository(r.Context(), project.ID, parts[2])
	if err != nil {
		writeOCIError(w, http.StatusBadRequest, "NAME_INVALID", "invalid repository name")
		return "", "", false
	}
	return project.ID, repo.ID, true
}

func (s *Server) ociBlobStat(w http.ResponseWriter, r *http.Request, name, digest string) {
	if _, _, ok := s.ociRepo(w, r, name, rbac.PermProjectRead); !ok {
		return
	}
	info, err := s.oci.StatBlob(r.Context(), digest)
	if err != nil {
		writeOCIError(w, http.StatusNotFound, "BLOB_UNKNOWN", "blob unknown")
		return
	}
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) ociBlobGet(w http.ResponseWriter, r *http.Request, name, digest string) {
	if _, _, ok := s.ociRepo(w, r, name, rbac.PermProjectRead); !ok {
		return
	}
	rc, info, err := s.oci.GetBlob(r.Context(), digest)
	if err != nil {
		writeOCIError(w, http.StatusNotFound, "BLOB_UNKNOWN", "blob unknown")
		return
	}
	defer rc.Close()
	w.Header().Set("Docker-Content-Digest", info.Digest)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}

func (s *Server) ociBlobUploadStart(w http.ResponseWriter, r *http.Request, name string) {
	projectID, _, ok := s.ociRepo(w, r, name, rbac.PermProjectUpdate)
	if !ok {
		return
	}
	if digest := r.URL.Query().Get("digest"); digest != "" {
		size, _ := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
		if size == 0 {
			size = -1
		}
		d, _, err := s.oci.PutBlobMonolithic(r.Context(), r.Body, size)
		if err != nil {
			mapOCIError(w, err)
			return
		}
		w.Header().Set("Docker-Content-Digest", d)
		w.Header().Set("Location", "/v2/"+name+"/blobs/"+d)
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusCreated)
		return
	}
	uploadID, err := s.oci.StartUpload(projectID, name)
	if err != nil {
		writeOCIError(w, http.StatusInternalServerError, "UNSUPPORTED", "internal error")
		return
	}
	w.Header().Set("Location", "/v2/"+name+"/blobs/uploads/"+uploadID)
	w.Header().Set("Range", "0-0")
	w.Header().Set("Docker-Upload-UUID", uploadID)
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) ociBlobUploadPatch(w http.ResponseWriter, r *http.Request, name, uploadID string) {
	if _, _, ok := s.ociRepo(w, r, name, rbac.PermProjectUpdate); !ok {
		return
	}
	offset, err := s.oci.AppendUpload(uploadID, r.Body)
	if err != nil {
		if errors.Is(err, identity.ErrNotFound) {
			writeOCIError(w, http.StatusNotFound, "BLOB_UPLOAD_UNKNOWN", "upload unknown")
			return
		}
		writeOCIError(w, http.StatusInternalServerError, "UNSUPPORTED", "internal error")
		return
	}
	end := offset - 1
	if end < 0 {
		end = 0
	}
	w.Header().Set("Location", "/v2/"+name+"/blobs/uploads/"+uploadID)
	w.Header().Set("Range", "0-"+strconv.FormatInt(end, 10))
	w.Header().Set("Docker-Upload-UUID", uploadID)
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) ociBlobUploadComplete(w http.ResponseWriter, r *http.Request, name, uploadID string) {
	if _, _, ok := s.ociRepo(w, r, name, rbac.PermProjectUpdate); !ok {
		return
	}
	digest, _, err := s.oci.CompleteUpload(r.Context(), uploadID, r.Body, r.URL.Query().Get("digest"))
	if err != nil {
		mapOCIError(w, err)
		return
	}
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Location", "/v2/"+name+"/blobs/"+digest)
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) ociManifestPut(w http.ResponseWriter, r *http.Request, name, ref string) {
	_, repoID, ok := s.ociRepo(w, r, name, rbac.PermProjectUpdate)
	if !ok {
		return
	}
	raw, err := readBodyLimited(r, 8<<20)
	if err != nil {
		writeOCIError(w, http.StatusBadRequest, "MANIFEST_UNKNOWN", "invalid body")
		return
	}
	mediaType := r.Header.Get("Content-Type")
	m, err := s.registry.PutManifest(r.Context(), repoID, mediaType, ref, raw)
	if err != nil {
		if errors.Is(err, identity.ErrInvalidInput) {
			writeOCIError(w, http.StatusBadRequest, "DIGEST_INVALID", err.Error())
			return
		}
		writeOCIError(w, http.StatusInternalServerError, "UNSUPPORTED", "internal error")
		return
	}
	w.Header().Set("Docker-Content-Digest", m.Digest)
	w.Header().Set("Location", "/v2/"+name+"/manifests/"+ref)
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) ociManifestGet(w http.ResponseWriter, r *http.Request, name, ref string) {
	_, repoID, ok := s.ociRepo(w, r, name, rbac.PermProjectRead)
	if !ok {
		return
	}
	m, raw, err := s.registry.GetManifest(r.Context(), repoID, ref)
	if err != nil {
		writeOCIError(w, http.StatusNotFound, "MANIFEST_UNKNOWN", "manifest unknown")
		return
	}
	w.Header().Set("Content-Type", m.MediaType)
	w.Header().Set("Docker-Content-Digest", m.Digest)
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func (s *Server) ociTagsList(w http.ResponseWriter, r *http.Request, name string) {
	_, repoID, ok := s.ociRepo(w, r, name, rbac.PermProjectRead)
	if !ok {
		return
	}
	tags, err := s.registry.ListTags(r.Context(), repoID)
	if err != nil {
		writeOCIError(w, http.StatusInternalServerError, "UNSUPPORTED", "internal error")
		return
	}
	if tags == nil {
		tags = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "tags": tags})
}
