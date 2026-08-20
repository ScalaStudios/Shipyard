package httpapi

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/packages"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/rbac"
)

func (s *Server) packageRepoAccess(w http.ResponseWriter, r *http.Request, format string, perm rbac.Permission) (repoID string, ok bool) {
	user, scopes, err := s.authenticate(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	required := identity.ScopeRegistryRead
	if perm != rbac.PermProjectRead {
		required = identity.ScopeRegistryWrite
	}
	if !identity.ScopeAllows(scopes, required) {
		writeError(w, http.StatusForbidden, "token missing "+required+" scope")
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
	if idx := strings.Index(name, "/-/"); idx >= 0 {
		s.serveNPMTarball(w, r, repoID, name[:idx], name[idx+len("/-/"):])
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
	base := path.Base(name)
	latest := ""
	vers := map[string]any{}
	for _, v := range versions {
		tarball := s.apiBaseURL(r) + "/repository/npm/" + r.PathValue("org") + "/" + r.PathValue("project") + "/" + r.PathValue("repo") + "/" + name + "/-/" + base + "-" + v.Version + ".tgz"
		obj := map[string]any{}
		if len(v.Metadata) > 0 {
			_ = json.Unmarshal(v.Metadata, &obj)
		}
		if len(obj) == 0 {
			obj = map[string]any{"name": name, "version": v.Version}
		}
		obj["name"] = name
		obj["version"] = v.Version
		obj["dist"] = map[string]any{
			"tarball":   tarball,
			"integrity": packages.NPMIntegrity(v.Digest),
		}
		vers[v.Version] = obj
		if latest == "" || packages.CompareVersions(v.Version, latest) > 0 {
			latest = v.Version
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":      name,
		"versions":  vers,
		"dist-tags": map[string]string{"latest": latest},
	})
}

func (s *Server) handleNPMTarball(w http.ResponseWriter, r *http.Request) {
	repoID, ok := s.packageRepoAccess(w, r, "npm", rbac.PermProjectRead)
	if !ok {
		return
	}
	s.serveNPMTarball(w, r, repoID, r.PathValue("name"), r.PathValue("filename"))
}

func (s *Server) serveNPMTarball(w http.ResponseWriter, r *http.Request, repoID, name, filename string) {
	base := path.Base(name)
	version := strings.TrimSuffix(strings.TrimPrefix(filename, base+"-"), ".tgz")
	if version == "" || version == filename {
		writeError(w, http.StatusBadRequest, "invalid tarball name")
		return
	}
	v, err := s.packages.GetVersion(r.Context(), repoID, name, version, "")
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
	var manifest json.RawMessage
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
		manifest = parsed.Manifest
	} else {
		data = body
	}
	if name == "" || version == "" {
		writeError(w, http.StatusBadRequest, "name and version required")
		return
	}
	v, err := s.packages.Publish(r.Context(), repoID, name, version, "", bytes.NewReader(data), int64(len(data)), manifest)
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
	base := path.Base(artifactPath)
	size, err := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		size = -1
	}
	if strings.HasPrefix(base, "maven-metadata.xml") {
		if _, err := s.packages.PublishOrReplace(r.Context(), repoID, artifactPath, "metadata", base, r.Body, size); err != nil {
			mapIdentityError(w, err)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	}
	name, version := mavenCoords(artifactPath)
	v, err := s.packages.PublishOrReplace(r.Context(), repoID, name, version, base, r.Body, size)
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
	base := path.Base(artifactPath)

	if v, err := s.packages.GetVersion(r.Context(), repoID, artifactPath, "metadata", base); err == nil {
		s.serveMavenBlob(w, r, v)
		return
	}

	if base == "maven-metadata.xml" || base == "maven-metadata.xml.sha1" || base == "maven-metadata.xml.md5" {
		xmlBytes, okMeta := s.mavenMetadataXML(w, r, repoID, artifactPath)
		if !okMeta {
			return
		}
		switch base {
		case "maven-metadata.xml.sha1":
			sum := sha1.Sum(xmlBytes)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, hex.EncodeToString(sum[:]))
		case "maven-metadata.xml.md5":
			sum := md5.Sum(xmlBytes)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, hex.EncodeToString(sum[:]))
		default:
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(xmlBytes)
		}
		return
	}

	name, version := mavenCoords(artifactPath)
	v, err := s.packages.GetVersion(r.Context(), repoID, name, version, base)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	s.serveMavenBlob(w, r, v)
}

func (s *Server) mavenMetadataXML(w http.ResponseWriter, r *http.Request, repoID, artifactPath string) ([]byte, bool) {
	metaPath := strings.TrimSuffix(strings.TrimSuffix(artifactPath, ".sha1"), ".md5")
	groupID, artifactID, okMeta := packages.MavenMetadataCoords(metaPath)
	if !okMeta {
		writeError(w, http.StatusNotFound, "not found")
		return nil, false
	}
	name := groupID + ":" + artifactID
	versions, err := s.packages.ListVersionsByName(r.Context(), repoID, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return nil, false
	}
	var vers []string
	updatedAt := time.Time{}
	for _, v := range versions {
		vers = append(vers, v.Version)
		if v.CreatedAt.After(updatedAt) {
			updatedAt = v.CreatedAt
		}
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	xmlBytes, err := packages.BuildMavenMetadata(groupID, artifactID, vers, updatedAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return nil, false
	}
	return xmlBytes, true
}

func (s *Server) serveMavenBlob(w http.ResponseWriter, r *http.Request, v packages.Version) {
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
