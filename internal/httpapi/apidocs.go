package httpapi

import (
	_ "embed"
	"net/http"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed openapi.yaml
var openapiYAML []byte

var (
	openapiOnce sync.Once
	openapiDoc  any
	openapiErr  error
)

func parsedOpenAPI() (any, error) {
	openapiOnce.Do(func() {
		openapiErr = yaml.Unmarshal(openapiYAML, &openapiDoc)
	})
	return openapiDoc, openapiErr
}

func (s *Server) handleOpenAPIYAML(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(openapiYAML)
}

func (s *Server) handleOpenAPIJSON(w http.ResponseWriter, _ *http.Request) {
	doc, err := parsedOpenAPI()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid openapi spec")
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

const apiDocsHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Shipyard API</title>
</head>
<body>
<script id="api-reference" data-url="/api/v1/openapi.json"></script>
<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`

func (s *Server) handleAPIDocs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(apiDocsHTML))
}
