package packages

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"time"
)

type mavenMetadata struct {
	XMLName    xml.Name `xml:"metadata"`
	GroupID    string   `xml:"groupId"`
	ArtifactID string   `xml:"artifactId"`
	Versioning struct {
		Latest      string `xml:"latest"`
		Release     string `xml:"release"`
		Versions    struct {
			Version []string `xml:"version"`
		} `xml:"versions"`
		LastUpdated string `xml:"lastUpdated"`
	} `xml:"versioning"`
}

func BuildMavenMetadata(groupID, artifactID string, versions []string) ([]byte, error) {
	versions = uniqueSorted(versions)
	meta := mavenMetadata{
		GroupID:    groupID,
		ArtifactID: artifactID,
	}
	meta.Versioning.Versions.Version = versions
	if len(versions) > 0 {
		latest := versions[len(versions)-1]
		meta.Versioning.Latest = latest
		meta.Versioning.Release = latest
	}
	meta.Versioning.LastUpdated = time.Now().UTC().Format("20060102150405")
	out, err := xml.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}

func SplitMavenName(name string) (groupID, artifactID string, ok bool) {
	parts := strings.SplitN(name, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func MavenMetadataCoords(artifactPath string) (groupID, artifactID string, ok bool) {
	path := strings.Trim(artifactPath, "/")
	if !strings.HasSuffix(path, "maven-metadata.xml") {
		return "", "", false
	}
	dir := strings.TrimSuffix(path, "/maven-metadata.xml")
	dir = strings.TrimSuffix(dir, "maven-metadata.xml")
	dir = strings.Trim(dir, "/")
	parts := strings.Split(dir, "/")
	if len(parts) < 2 {
		return "", "", false
	}
	artifactID = parts[len(parts)-1]
	groupID = strings.Join(parts[:len(parts)-1], ".")
	return groupID, artifactID, true
}

func uniqueSorted(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func FormatMavenError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("maven metadata: %w", err)
}
