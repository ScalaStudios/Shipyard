package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/rbac"
)

type runnerInstallRequest struct {
	OrganizationID string `json:"organization_id"`
	TTL            string `json:"ttl"`
	Name           string `json:"name"`
	Labels         string `json:"labels"`
}

func (s *Server) apiBaseURL(r *http.Request) string {
	if u := s.apiURL(r.Context()); u != "" {
		return u
	}
	// Prefer reverse-proxied public origin when it is not a Vite-only URL.
	pub := s.publicURL(r.Context())
	if pub != "" && !strings.Contains(pub, ":5173") && !strings.Contains(pub, ":5174") {
		return pub
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	if host != "" && !strings.Contains(host, ":5173") && !strings.Contains(host, ":5174") {
		return scheme + "://" + host
	}
	return "http://127.0.0.1:8080"
}

func (s *Server) requireRunnerScope(w http.ResponseWriter, r *http.Request, orgID string) bool {
	if orgID == "" {
		if !currentUser(r).IsAdmin {
			writeError(w, http.StatusForbidden, "organization_id required")
			return false
		}
		return true
	}
	if _, _, err := s.orgs.Require(r.Context(), currentUser(r).ID, orgID, rbac.PermOrgUpdate); err != nil {
		mapIdentityError(w, err)
		return false
	}
	return true
}

func (s *Server) handleCreateRunnerInstall(w http.ResponseWriter, r *http.Request) {
	var req runnerInstallRequest
	_ = decodeJSON(r, &req)
	ttl := 24 * time.Hour
	if req.TTL != "" {
		if d, err := time.ParseDuration(req.TTL); err == nil {
			ttl = d
		}
	}
	if !s.requireRunnerScope(w, r, req.OrganizationID) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "runner-1"
	}
	labels := strings.TrimSpace(req.Labels)
	if labels == "" {
		labels = "linux"
	}
	if !safeInstallParam(name) || !safeInstallParam(labels) {
		writeError(w, http.StatusBadRequest, "invalid characters")
		return
	}

	token, expires, err := s.runners.CreateRegistrationToken(r.Context(), req.OrganizationID, currentUser(r).ID, ttl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	apiURL := s.apiBaseURL(r)
	q := url.Values{}
	q.Set("token", token)
	q.Set("name", name)
	q.Set("labels", labels)
	q.Set("url", apiURL)
	scriptURL := apiURL + "/api/v1/runners/install.sh?" + q.Encode()

	writeJSON(w, http.StatusCreated, map[string]any{
		"token":          token,
		"expires_at":     expires,
		"api_url":        apiURL,
		"name":           name,
		"labels":         labels,
		"script_url":     scriptURL,
		"curl_command":   fmt.Sprintf("curl -fsSL %q | bash", scriptURL),
		"docker_command": runnerDockerCommand(apiURL, token, name, labels),
		"manual_env":     runnerManualEnv(apiURL, token, name, labels),
		"systemd_hint":   "After auto-install, a user systemd unit shipyard-runner.service is enabled when systemctl --user is available.",
	})
}

func (s *Server) handleRunnerInstallScript(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}
	ok, err := s.runners.PeekRegistrationToken(r.Context(), token)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "invalid or expired registration token", http.StatusUnauthorized)
		return
	}

	apiURL := r.URL.Query().Get("url")
	if apiURL == "" {
		apiURL = s.apiBaseURL(r)
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "runner-1"
	}
	labels := r.URL.Query().Get("labels")
	if labels == "" {
		labels = "linux"
	}
	method := r.URL.Query().Get("method") // docker|binary|auto
	if !safeInstallParam(apiURL) || !safeInstallParam(name) || !safeInstallParam(labels) || !safeInstallParam(method) {
		http.Error(w, "invalid characters", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="shipyard-runner-install.sh"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(renderRunnerInstallScript(apiURL, token, name, labels, method)))
}

func safeInstallParam(v string) bool {
	for _, c := range v {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.', c == '_', c == ':', c == '/', c == ',', c == '@', c == '-':
		default:
			return false
		}
	}
	return true
}

func runnerDockerCommand(apiURL, token, name, labels string) string {
	q := url.Values{}
	q.Set("token", token)
	q.Set("name", name)
	q.Set("labels", labels)
	q.Set("url", apiURL)
	q.Set("method", "docker")
	return fmt.Sprintf("curl -fsSL %q | bash", apiURL+"/api/v1/runners/install.sh?"+q.Encode())
}

func runnerManualEnv(apiURL, token, name, labels string) string {
	return fmt.Sprintf(`export SHIPYARD_URL=%q
export SHIPYARD_REGISTRATION_TOKEN=%q
export SHIPYARD_RUNNER_NAME=%q
export SHIPYARD_RUNNER_LABELS=%q
export SHIPYARD_RUNNER_TOKEN_FILE="$HOME/.shipyard/runner.token"
go install git.lunarlabs.dev/Shipyard/shipyard/cmd/shipyard-runner@master
shipyard-runner`, apiURL, token, name, labels)
}

func renderRunnerInstallScript(apiURL, token, name, labels, method string) string {
	apiURL = strings.TrimRight(apiURL, "/")
	if method == "" {
		method = "auto"
	}
	return fmt.Sprintf(`#!/usr/bin/env bash
# Shipyard runner installer (generated by control plane — Pterodactyl-style one-liner)
# curl -fsSL "%s/api/v1/runners/install.sh?..." | bash
set -euo pipefail

SHIPYARD_URL=%q
SHIPYARD_REGISTRATION_TOKEN=%q
SHIPYARD_RUNNER_NAME=%q
SHIPYARD_RUNNER_LABELS=%q
METHOD=%q
SHIPYARD_HOME="${SHIPYARD_HOME:-$HOME/.shipyard}"
TOKEN_FILE="$SHIPYARD_HOME/runner.token"
BIN_DIR="$SHIPYARD_HOME/bin"
UNIT_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"

log() { printf '==> %%s\n' "$*"; }
die() { printf '!!  %%s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

mkdir -p "$SHIPYARD_HOME" "$BIN_DIR"
chmod 700 "$SHIPYARD_HOME"

install_docker() {
  have docker || return 1
  log "Installing runner via Docker"
  local image="shipyard-runner:local"
  local tmp
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' RETURN
  if have git; then
    git clone --depth 1 https://git.lunarlabs.dev/Shipyard/shipyard.git "$tmp/src"
  else
    die "git required to build the runner image (or install Go and re-run with METHOD=binary)"
  fi
  docker build -t "$image" -f "$tmp/src/deploy/compose/Dockerfile.runner" "$tmp/src"
  docker rm -f "shipyard-${SHIPYARD_RUNNER_NAME}" >/dev/null 2>&1 || true
  docker volume create shipyard-runner-data >/dev/null
  docker run -d --restart unless-stopped \
    --name "shipyard-${SHIPYARD_RUNNER_NAME}" \
    -e SHIPYARD_URL="$SHIPYARD_URL" \
    -e SHIPYARD_REGISTRATION_TOKEN="$SHIPYARD_REGISTRATION_TOKEN" \
    -e SHIPYARD_RUNNER_NAME="$SHIPYARD_RUNNER_NAME" \
    -e SHIPYARD_RUNNER_LABELS="$SHIPYARD_RUNNER_LABELS" \
    -e SHIPYARD_RUNNER_TOKEN_FILE=/var/lib/shipyard/runner.token \
    -v shipyard-runner-data:/var/lib/shipyard \
    --label shipyard.runner=true \
    "$image"
  log "Runner container shipyard-${SHIPYARD_RUNNER_NAME} is up"
  docker logs --tail 20 "shipyard-${SHIPYARD_RUNNER_NAME}" || true
}

install_binary() {
  have go || die "Go toolchain not found (install Go 1.25+ or use Docker)"
  log "Installing shipyard-runner with go install"
  GOBIN="$BIN_DIR" go install git.lunarlabs.dev/Shipyard/shipyard/cmd/shipyard-runner@master
  export PATH="$BIN_DIR:$PATH"
  cat > "$SHIPYARD_HOME/runner.env" <<EOF
SHIPYARD_URL=$SHIPYARD_URL
SHIPYARD_RUNNER_NAME=$SHIPYARD_RUNNER_NAME
SHIPYARD_RUNNER_LABELS=$SHIPYARD_RUNNER_LABELS
SHIPYARD_RUNNER_TOKEN_FILE=$TOKEN_FILE
SHIPYARD_REGISTRATION_TOKEN=$SHIPYARD_REGISTRATION_TOKEN
EOF
  chmod 600 "$SHIPYARD_HOME/runner.env"

  if have systemctl && systemctl --user status >/dev/null 2>&1; then
    mkdir -p "$UNIT_DIR"
    cat > "$UNIT_DIR/shipyard-runner.service" <<EOF
[Unit]
Description=Shipyard runner (${SHIPYARD_RUNNER_NAME})
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=$SHIPYARD_HOME/runner.env
ExecStart=$BIN_DIR/shipyard-runner
Restart=always
RestartSec=3

[Install]
WantedBy=default.target
EOF
    systemctl --user daemon-reload
    systemctl --user enable --now shipyard-runner.service
    log "Enabled systemd user unit: systemctl --user status shipyard-runner"
  else
    log "Starting runner in background (no user systemd)"
    nohup env $(grep -v '^#' "$SHIPYARD_HOME/runner.env" | xargs) "$BIN_DIR/shipyard-runner" \
      >"$SHIPYARD_HOME/runner.log" 2>&1 &
    echo $! >"$SHIPYARD_HOME/runner.pid"
    log "PID $(cat "$SHIPYARD_HOME/runner.pid") — logs: $SHIPYARD_HOME/runner.log"
  fi
}

case "$METHOD" in
  docker) install_docker || die "Docker install failed" ;;
  binary) install_binary ;;
  auto)
    if have docker; then
      install_docker || { log "Docker path failed; falling back to binary"; install_binary; }
    else
      install_binary
    fi
    ;;
  *) die "unknown METHOD=$METHOD (use auto|docker|binary)" ;;
esac

log "Done. Check the Shipyard UI → Runners for heartbeat."
`, apiURL, apiURL, token, name, labels, method)
}
