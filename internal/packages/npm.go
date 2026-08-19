package packages

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
)

type NPMPublishRequest struct {
	Name     string
	Version  string
	Data     []byte
	Manifest json.RawMessage
}

type npmPublishBody struct {
	Name        string                     `json:"name"`
	Versions    map[string]json.RawMessage `json:"versions"`
	DistTags    map[string]string          `json:"dist-tags"`
	Attachments map[string]struct {
		ContentType string `json:"content_type"`
		Data        string `json:"data"`
		Length      int    `json:"length"`
	} `json:"_attachments"`
}

func ParseNPMPublish(body []byte) (NPMPublishRequest, error) {
	var req npmPublishBody
	if err := json.Unmarshal(body, &req); err != nil {
		return NPMPublishRequest{}, fmt.Errorf("%w: invalid npm publish json", identity.ErrInvalidInput)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return NPMPublishRequest{}, fmt.Errorf("%w: package name required", identity.ErrInvalidInput)
	}
	version := ""
	if req.DistTags != nil {
		version = req.DistTags["latest"]
	}
	if version == "" {
		for v := range req.Versions {
			version = v
			break
		}
	}
	if version == "" {
		return NPMPublishRequest{}, fmt.Errorf("%w: version required", identity.ErrInvalidInput)
	}
	if len(req.Attachments) == 0 {
		return NPMPublishRequest{}, fmt.Errorf("%w: _attachments required", identity.ErrInvalidInput)
	}
	var data []byte
	for _, att := range req.Attachments {
		raw, err := base64.StdEncoding.DecodeString(att.Data)
		if err != nil {
			return NPMPublishRequest{}, fmt.Errorf("%w: invalid attachment encoding", identity.ErrInvalidInput)
		}
		data = raw
		break
	}
	if len(data) == 0 {
		return NPMPublishRequest{}, fmt.Errorf("%w: empty attachment", identity.ErrInvalidInput)
	}
	return NPMPublishRequest{Name: name, Version: version, Data: data, Manifest: req.Versions[version]}, nil
}

func NPMIntegrity(digest string) string {
	b, err := hex.DecodeString(strings.TrimPrefix(digest, "sha256:"))
	if err != nil {
		return ""
	}
	return "sha256-" + base64.StdEncoding.EncodeToString(b)
}

func IsNPMPublishJSON(contentType string, body []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "json") {
		return true
	}
	trim := bytes.TrimSpace(body)
	return len(trim) > 0 && trim[0] == '{'
}
