package httpapi

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
)

func (s *Server) handleOCIAPIVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleOCIGet(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(r.PathValue("rest"), "/")
	if rest == "" {
		s.handleOCIAPIVersion(w, r)
		return
	}
	s.requireRegistryAuth(s.handleOCIDistribution)(w, r)
}

func (s *Server) handleOCIDistribution(w http.ResponseWriter, r *http.Request) {
	kind, name, a := parseOCIRest(r.PathValue("rest"))
	switch kind {
	case "blob":
		if r.Method == http.MethodHead {
			s.ociBlobExists(w, r, name, a)
			return
		}
		if r.Method == http.MethodGet {
			s.ociBlobGet(w, r, name, a)
			return
		}
	case "upload-start":
		if r.Method == http.MethodPost {
			s.ociBlobUploadStart(w, r, name)
			return
		}
	case "upload-complete":
		if r.Method == http.MethodPut {
			s.ociBlobUploadComplete(w, r, name, a)
			return
		}
	case "manifest":
		if r.Method == http.MethodPut {
			s.ociManifestPut(w, r, name, a)
			return
		}
		if r.Method == http.MethodGet {
			s.ociManifestGet(w, r, name, a)
			return
		}
	}
	writeError(w, http.StatusNotFound, "not found")
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
	if idx := strings.Index(rest, "/blobs/"); idx >= 0 {
		return "blob", rest[:idx], rest[idx+len("/blobs/"):]
	}
	if idx := strings.Index(rest, "/manifests/"); idx >= 0 {
		return "manifest", rest[:idx], rest[idx+len("/manifests/"):]
	}
	return "", "", ""
}

func (s *Server) ociBlobExists(w http.ResponseWriter, r *http.Request, name, digest string) {
	if _, _, err := s.oci.ResolveProjectRepo(r.Context(), name); err != nil {
		mapIdentityError(w, err)
		return
	}
	ok, err := s.oci.BlobExists(r.Context(), digest)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Docker-Content-Digest", digest)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) ociBlobGet(w http.ResponseWriter, r *http.Request, name, digest string) {
	if _, _, err := s.oci.ResolveProjectRepo(r.Context(), name); err != nil {
		mapIdentityError(w, err)
		return
	}
	rc, info, err := s.oci.GetBlob(r.Context(), digest)
	if err != nil {
		mapIdentityError(w, identity.ErrNotFound)
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
	projectID, _, err := s.oci.ResolveProjectRepo(r.Context(), name)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	if digest := r.URL.Query().Get("digest"); digest != "" {
		size, _ := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
		if size == 0 {
			size = -1
		}
		d, n, err := s.oci.PutBlobMonolithic(r.Context(), r.Body, size)
		if err != nil {
			mapIdentityError(w, err)
			return
		}
		w.Header().Set("Docker-Content-Digest", d)
		w.Header().Set("Location", "/v2/"+name+"/blobs/"+d)
		w.Header().Set("Content-Length", strconv.FormatInt(n, 10))
		w.WriteHeader(http.StatusCreated)
		return
	}
	uploadID, err := s.oci.StartUpload(projectID, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.Header().Set("Location", "/v2/"+name+"/blobs/uploads/"+uploadID)
	w.Header().Set("Range", "0-0")
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) ociBlobUploadComplete(w http.ResponseWriter, r *http.Request, name, uploadID string) {
	if _, _, err := s.oci.ResolveProjectRepo(r.Context(), name); err != nil {
		mapIdentityError(w, err)
		return
	}
	size, err := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		size = -1
	}
	digest, n, err := s.oci.CompleteUpload(r.Context(), uploadID, r.Body, size)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	expected := r.URL.Query().Get("digest")
	if expected != "" && !strings.EqualFold(expected, digest) {
		writeError(w, http.StatusBadRequest, "digest mismatch")
		return
	}
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Location", "/v2/"+name+"/blobs/"+digest)
	w.Header().Set("Content-Length", strconv.FormatInt(n, 10))
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) ociManifestPut(w http.ResponseWriter, r *http.Request, name, ref string) {
	_, repoID, err := s.oci.ResolveProjectRepo(r.Context(), name)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	raw, err := readBodyLimited(r, 8<<20)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	mediaType := r.Header.Get("Content-Type")
	m, err := s.registry.PutManifest(r.Context(), repoID, mediaType, ref, raw)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	w.Header().Set("Docker-Content-Digest", m.Digest)
	w.Header().Set("Location", "/v2/"+name+"/manifests/"+ref)
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) ociManifestGet(w http.ResponseWriter, r *http.Request, name, ref string) {
	_, repoID, err := s.oci.ResolveProjectRepo(r.Context(), name)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	m, raw, err := s.registry.GetManifestByTag(r.Context(), repoID, ref)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	w.Header().Set("Content-Type", m.MediaType)
	w.Header().Set("Docker-Content-Digest", m.Digest)
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}
