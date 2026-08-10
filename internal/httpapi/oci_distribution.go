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

func (s *Server) handleOCIBlobExists(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	digest := r.PathValue("digest")
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

func (s *Server) handleOCIBlobGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	digest := r.PathValue("digest")
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

func (s *Server) handleOCIBlobUploadStart(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
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

func (s *Server) handleOCIBlobUploadComplete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	uploadID := r.PathValue("uuid")
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

func (s *Server) handleOCIManifestPutDist(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	ref := r.PathValue("reference")
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

func (s *Server) handleOCIManifestGetDist(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	ref := r.PathValue("reference")
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
