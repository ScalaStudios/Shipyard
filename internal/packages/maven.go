package packages

import (
	"encoding/xml"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

type mavenMetadata struct {
	XMLName    xml.Name `xml:"metadata"`
	GroupID    string   `xml:"groupId"`
	ArtifactID string   `xml:"artifactId"`
	Versioning struct {
		Latest   string `xml:"latest"`
		Release  string `xml:"release"`
		Versions struct {
			Version []string `xml:"version"`
		} `xml:"versions"`
		LastUpdated string `xml:"lastUpdated"`
	} `xml:"versioning"`
}

func BuildMavenMetadata(groupID, artifactID string, versions []string, updatedAt time.Time) ([]byte, error) {
	versions = uniqueSorted(versions)
	meta := mavenMetadata{
		GroupID:    groupID,
		ArtifactID: artifactID,
	}
	meta.Versioning.Versions.Version = versions
	if len(versions) > 0 {
		meta.Versioning.Latest = versions[len(versions)-1]
		for i := len(versions) - 1; i >= 0; i-- {
			if !strings.HasSuffix(strings.ToUpper(versions[i]), "-SNAPSHOT") {
				meta.Versioning.Release = versions[i]
				break
			}
		}
	}
	meta.Versioning.LastUpdated = updatedAt.UTC().Format("20060102150405")
	out, err := xml.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}

func CompareVersions(a, b string) int {
	baseA, snapA := trimSnapshot(a)
	baseB, snapB := trimSnapshot(b)
	if c := compareVersionBase(baseA, baseB); c != 0 {
		return c
	}
	if snapA == snapB {
		return 0
	}
	if snapA {
		return -1
	}
	return 1
}

func trimSnapshot(v string) (string, bool) {
	const suffix = "-SNAPSHOT"
	if len(v) >= len(suffix) && strings.EqualFold(v[len(v)-len(suffix):], suffix) {
		return v[:len(v)-len(suffix)], true
	}
	return v, false
}

func compareVersionBase(a, b string) int {
	sa := splitVersion(a)
	sb := splitVersion(b)
	n := len(sa)
	if len(sb) > n {
		n = len(sb)
	}
	for i := 0; i < n; i++ {
		x, y := "0", "0"
		if i < len(sa) {
			x = sa[i]
		}
		if i < len(sb) {
			y = sb[i]
		}
		xi, xErr := strconv.Atoi(x)
		yi, yErr := strconv.Atoi(y)
		if xErr == nil && yErr == nil {
			if xi != yi {
				if xi < yi {
					return -1
				}
				return 1
			}
			continue
		}
		lx := strings.ToLower(x)
		ly := strings.ToLower(y)
		if lx != ly {
			if lx < ly {
				return -1
			}
			return 1
		}
	}
	return 0
}

func splitVersion(v string) []string {
	return strings.FieldsFunc(v, func(r rune) bool { return r == '.' || r == '-' })
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
	slices.SortFunc(out, CompareVersions)
	return out
}

func FormatMavenError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("maven metadata: %w", err)
}
