package httpapi

import (
	"bytes"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/packages"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/rbac"
)

func (s *Server) packageRepoAccess(w http.ResponseWriter, r *http.Request, format string, perm rbac.Permission) (repoID string, ok bool) {
	user, err := s.authenticate(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	_, project, err := s.orgs.RequireBySlug(r.Context(), user.ID, r.PathValue("org"), r.PathValue("project"), perm)
	if err != nil {
		mapIdentityError(w, err)
		return "", false
	}
	repo, err := s.packages.GetRepositoryByName(r.Context(), project.ID, r.PathValue("repo"), format)
	if err != nil {
		mapIdentityError(w, err)
		return "", false
	}
	return repo.ID, true
}

func (s *Server) handleNPMPackument(w http.ResponseWriter, r *http.Request) {
	repoID, ok := s.packageRepoAccess(w, r, "npm", rbac.PermProjectRead)
	if !ok {
		return
	}
	name := strings.TrimPrefix(r.PathValue("name"), "/")
	name = strings.Trim(name, "/")
	if name == "" {
		writeError(w, http.StatusBadRequest, "package name required")
		return
	}
	versions, err := s.packages.ListVersionsByName(r.Context(), repoID, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if len(versions) == 0 {
		http.NotFound(w, r)
		return
	}
	distTags := map[string]string{"latest": versions[0].Version}
	vers := map[string]any{}
	for _, v := range versions {
		tarball := "/repository/npm/" + r.PathValue("org") + "/" + r.PathValue("project") + "/" + r.PathValue("repo") + "/" + name + "/-/" + name + "-" + v.Version + ".tgz"
		vers[v.Version] = map[string]any{
			"name":    name,
			"version": v.Version,
			"dist": map[string]any{
				"tarball": tarball,
				"shasum":  strings.TrimPrefix(v.Digest, "sha256:"),
			},
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":      name,
		"versions":  vers,
		"dist-tags": distTags,
	})
}

func (s *Server) handleNPMTarball(w http.ResponseWriter, r *http.Request) {
	repoID, ok := s.packageRepoAccess(w, r, "npm", rbac.PermProjectRead)
	if !ok {
		return
	}
	name := r.PathValue("name")
	filename := r.PathValue("filename")
	version := strings.TrimSuffix(strings.TrimPrefix(filename, name+"-"), ".tgz")
	if version == "" || version == filename {
		writeError(w, http.StatusBadRequest, "invalid tarball name")
		return
	}
	v, err := s.packages.GetVersion(r.Context(), repoID, name, version)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	rc, info, err := s.packages.Open(r.Context(), v.Digest)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	w.Header().Set("Digest", info.Digest)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}

func (s *Server) handleNPMPublish(w http.ResponseWriter, r *http.Request) {
	repoID, ok := s.packageRepoAccess(w, r, "npm", rbac.PermProjectUpdate)
	if !ok {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	name := strings.Trim(r.PathValue("name"), "/")
	version := r.URL.Query().Get("version")
	if version == "" {
		version = r.Header.Get("X-Shipyard-Version")
	}

	var data []byte
	if packages.IsNPMPublishJSON(r.Header.Get("Content-Type"), body) {
		parsed, err := packages.ParseNPMPublish(body)
		if err != nil {
			mapIdentityError(w, err)
			return
		}
		if name == "" {
			name = parsed.Name
		}
		if version == "" {
			version = parsed.Version
		}
		data = parsed.Data
	} else {
		data = body
	}
	if name == "" || version == "" {
		writeError(w, http.StatusBadRequest, "name and version required")
		return
	}
	v, err := s.packages.Publish(r.Context(), repoID, name, version, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "version": v})
}

func (s *Server) handleMavenPut(w http.ResponseWriter, r *http.Request) {
	repoID, ok := s.packageRepoAccess(w, r, "maven", rbac.PermProjectUpdate)
	if !ok {
		return
	}
	artifactPath := strings.Trim(r.PathValue("path"), "/")
	if artifactPath == "" {
		writeError(w, http.StatusBadRequest, "path required")
		return
	}
	if strings.HasSuffix(artifactPath, "maven-metadata.xml") {
		w.WriteHeader(http.StatusCreated)
		return
	}
	name, version := mavenCoords(artifactPath)
	size, err := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		size = -1
	}
	v, err := s.packages.Publish(r.Context(), repoID, name, version, r.Body, size)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"version": v})
}

func (s *Server) handleMavenGet(w http.ResponseWriter, r *http.Request) {
	repoID, ok := s.packageRepoAccess(w, r, "maven", rbac.PermProjectRead)
	if !ok {
		return
	}
	artifactPath := strings.Trim(r.PathValue("path"), "/")
	if groupID, artifactID, okMeta := packages.MavenMetadataCoords(artifactPath); okMeta {
		name := groupID + ":" + artifactID
		versions, err := s.packages.ListVersionsByName(r.Context(), repoID, name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		var vers []string
		for _, v := range versions {
			vers = append(vers, v.Version)
		}
		xmlBytes, err := packages.BuildMavenMetadata(groupID, artifactID, vers)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(xmlBytes)
		return
	}
	name, version := mavenCoords(artifactPath)
	v, err := s.packages.GetVersion(r.Context(), repoID, name, version)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	rc, info, err := s.packages.Open(r.Context(), v.Digest)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	w.Header().Set("Digest", info.Digest)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}

func mavenCoords(artifactPath string) (name, version string) {
	base := path.Base(artifactPath)
	dir := path.Dir(artifactPath)
	version = path.Base(dir)
	artifact := path.Base(path.Dir(dir))
	group := strings.ReplaceAll(path.Dir(path.Dir(dir)), "/", ".")
	name = group + ":" + artifact
	if strings.HasPrefix(base, artifact+"-"+version) {
		return name, version
	}
	return artifactPath, "file"
}
