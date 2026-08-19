package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/buildkit"
)

func main() {
	serverURL := envOr("SHIPYARD_URL", "http://127.0.0.1:8080")
	token := os.Getenv("SHIPYARD_RUNNER_TOKEN")
	tokenFile := envOr("SHIPYARD_RUNNER_TOKEN_FILE", "")
	regToken := os.Getenv("SHIPYARD_REGISTRATION_TOKEN")
	name := envOr("SHIPYARD_RUNNER_NAME", hostname())
	labels := runnerLabels(envOr("SHIPYARD_RUNNER_LABELS", runtime.GOOS))

	if token == "" && tokenFile != "" {
		if b, err := os.ReadFile(tokenFile); err == nil {
			token = strings.TrimSpace(string(b))
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client := &http.Client{Timeout: 30 * time.Second}

	if token == "" {
		if regToken == "" {
			fatal("SHIPYARD_RUNNER_TOKEN or SHIPYARD_REGISTRATION_TOKEN required")
		}
		body, _ := json.Marshal(map[string]any{
			"token":        regToken,
			"name":         name,
			"labels":       labels,
			"capabilities": []string{},
		})
		resp, err := client.Post(serverURL+"/api/v1/runner/register", "application/json", bytes.NewReader(body))
		if err != nil {
			fatal(err.Error())
		}
		defer resp.Body.Close()
		var out struct {
			RunnerToken string `json:"runner_token"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		if resp.StatusCode >= 300 || out.RunnerToken == "" {
			fatal("registration failed")
		}
		token = out.RunnerToken
		if tokenFile != "" {
			_ = os.MkdirAll(filepath.Dir(tokenFile), 0o700)
			if err := os.WriteFile(tokenFile, []byte(token+"\n"), 0o600); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not persist runner token: %v\n", err)
			} else {
				fmt.Println("persisted runner token to", tokenFile)
			}
		}
		fmt.Println("registered runner")
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = postJSON(client, serverURL+"/api/v1/runner/heartbeat", token, map[string]any{})
			job, ok := leaseJob(client, serverURL, token)
			if !ok {
				continue
			}
			runJob(ctx, client, serverURL, token, job)
		}
	}
}

type leaseJobResp struct {
	ID      string            `json:"id"`
	RunID   string            `json:"run_id"`
	Name    string            `json:"name"`
	LeaseID string            `json:"lease_id"`
	Secrets map[string]string `json:"secrets"`
	Env     map[string]string `json:"env"`
	Run     *struct {
		ID          string `json:"id"`
		Number      int64  `json:"number"`
		GitRef      string `json:"git_ref"`
		GitSHA      string `json:"git_sha"`
		ProjectSlug string `json:"project_slug"`
		OrgSlug     string `json:"org_slug"`
	} `json:"run"`
	Repo *struct {
		CloneURL string `json:"clone_url"`
		Username string `json:"username"`
		Token    string `json:"token"`
	} `json:"repo"`
}

func leaseJob(client *http.Client, serverURL, token string) (leaseJobResp, bool) {
	req, _ := http.NewRequest(http.MethodPost, serverURL+"/api/v1/runner/jobs/lease", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode == http.StatusNoContent {
		if resp != nil {
			resp.Body.Close()
		}
		return leaseJobResp{}, false
	}
	defer resp.Body.Close()
	var wrap struct {
		Job leaseJobResp `json:"job"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrap); err != nil || wrap.Job.ID == "" {
		return leaseJobResp{}, false
	}
	return wrap.Job, true
}

func runJob(ctx context.Context, client *http.Client, serverURL, token string, job leaseJobResp) {
	jobCtx, cancelJob := context.WithCancel(ctx)
	defer cancelJob()

	ws := filepath.Join(workRoot(), job.ID)
	_ = os.MkdirAll(ws, 0o700)

	var remoteCanceled atomic.Bool
	stopRenew := make(chan struct{})
	var renew sync.WaitGroup
	renew.Add(1)
	go func() {
		defer renew.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopRenew:
				return
			case <-jobCtx.Done():
				return
			case <-ticker.C:
				status, ok := renewLease(client, serverURL, token, job)
				if ok && status != "leased" && status != "running" {
					remoteCanceled.Store(true)
					cancelJob()
					return
				}
			}
		}
	}()

	_ = postJSON(client, serverURL+"/api/v1/runner/jobs/"+job.ID+"/start", token, map[string]any{"lease_id": job.LeaseID})
	secretVals := make([]string, 0, len(job.Secrets))
	for _, v := range job.Secrets {
		if v != "" {
			secretVals = append(secretVals, v)
		}
	}
	steps := fetchSteps(client, serverURL, token, job.ID)
	failed := false
	stopped := false
	for _, step := range steps {
		_ = postJSON(client, serverURL+"/api/v1/runner/steps/"+step.ID+"/status", token, map[string]any{"status": "running"})
		code, err := execStep(jobCtx, client, serverURL, token, job, ws, step, secretVals)
		status := "succeeded"
		if err != nil || code != 0 {
			status = "failed"
			failed = true
		}
		if jobCtx.Err() != nil {
			status = "canceled"
			stopped = true
		}
		_ = postJSON(client, serverURL+"/api/v1/runner/steps/"+step.ID+"/status", token, map[string]any{"status": status, "exit_code": code})
		if failed || stopped {
			break
		}
	}
	close(stopRenew)
	renew.Wait()

	final := "succeeded"
	errMsg := ""
	switch {
	case stopped && remoteCanceled.Load():
		final = "canceled"
	case stopped:
		final = "failed"
		errMsg = "runner stopped"
	case failed:
		final = "failed"
	}
	payload := map[string]any{"lease_id": job.LeaseID, "status": final}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	_ = postJSON(client, serverURL+"/api/v1/runner/jobs/"+job.ID+"/complete", token, payload)

	_ = os.Remove(askpassPath(job.ID))
	if os.Getenv("SHIPYARD_KEEP_WORKSPACE") != "true" {
		_ = os.RemoveAll(ws)
	}
}

func renewLease(client *http.Client, serverURL, token string, job leaseJobResp) (string, bool) {
	body, _ := json.Marshal(map[string]any{"lease_id": job.LeaseID})
	req, _ := http.NewRequest(http.MethodPost, serverURL+"/api/v1/runner/jobs/"+job.ID+"/renew", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	var out struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.Status == "" {
		return "", false
	}
	return out.Status, true
}

type step struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Uses string `json:"uses"`
	Run  string `json:"run_script"`
}

func fetchSteps(client *http.Client, serverURL, token, jobID string) []step {
	req, _ := http.NewRequest(http.MethodGet, serverURL+"/api/v1/runner/jobs/"+jobID+"/steps", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var wrap struct {
		Steps []step `json:"steps"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&wrap)
	return wrap.Steps
}

func execStep(ctx context.Context, client *http.Client, serverURL, token string, job leaseJobResp, ws string, st step, secrets []string) (int, error) {
	if st.Run == "" && st.Uses != "" {
		switch {
		case strings.HasPrefix(st.Uses, "shipyard/checkout"):
			return execCheckout(ctx, client, serverURL, token, job, ws, st, secrets)
		case strings.HasPrefix(st.Uses, "shipyard/build-image"):
			return execBuildImage(ctx, client, serverURL, token, job.ID, ws, st, secrets)
		default:
			_ = appendLog(client, serverURL, token, job.ID, st.ID, "system", "unsupported step: uses="+st.Uses, secrets)
			return 1, nil
		}
	}
	cmd := exec.CommandContext(ctx, "bash", "-lc", st.Run)
	cmd.Dir = ws
	cmd.Env = stepEnv(job, ws)
	cmd.WaitDelay = 5 * time.Second
	return runCommand(client, serverURL, token, job.ID, st.ID, cmd, secrets)
}

func stepEnv(job leaseJobResp, ws string) []string {
	runID := job.RunID
	number, gitRef, gitSHA, project, org := "", "", "", "", ""
	if job.Run != nil {
		if job.Run.ID != "" {
			runID = job.Run.ID
		}
		number = strconv.FormatInt(job.Run.Number, 10)
		gitRef = job.Run.GitRef
		gitSHA = job.Run.GitSHA
		project = job.Run.ProjectSlug
		org = job.Run.OrgSlug
	}
	env := append(os.Environ(),
		"CI=true",
		"SHIPYARD_RUN_ID="+runID,
		"SHIPYARD_RUN_NUMBER="+number,
		"SHIPYARD_JOB_ID="+job.ID,
		"SHIPYARD_JOB_NAME="+job.Name,
		"SHIPYARD_GIT_REF="+gitRef,
		"SHIPYARD_GIT_SHA="+gitSHA,
		"SHIPYARD_PROJECT="+project,
		"SHIPYARD_ORG="+org,
		"SHIPYARD_WORKSPACE="+ws,
	)
	for k, v := range job.Env {
		env = append(env, k+"="+v)
	}
	for k, v := range job.Secrets {
		env = append(env, k+"="+v)
	}
	return env
}

const askpassScript = `#!/bin/sh
case "$1" in
  *sername*) printf '%s\n' "$SHIPYARD_GIT_USERNAME" ;;
  *) printf '%s\n' "$SHIPYARD_GIT_TOKEN" ;;
esac
`

func execCheckout(ctx context.Context, client *http.Client, serverURL, token string, job leaseJobResp, ws string, st step, secrets []string) (int, error) {
	if job.Repo == nil || job.Repo.CloneURL == "" {
		_ = appendLog(client, serverURL, token, job.ID, st.ID, "system",
			"checkout: no forge connection for this project; add one under Settings › Integrations", secrets)
		return 1, nil
	}
	helper := askpassPath(job.ID)
	if err := os.WriteFile(helper, []byte(askpassScript), 0o700); err != nil {
		_ = appendLog(client, serverURL, token, job.ID, st.ID, "system", "checkout: "+err.Error(), secrets)
		return 1, err
	}
	env := append(os.Environ(),
		"GIT_ASKPASS="+helper,
		"GIT_TERMINAL_PROMPT=0",
		"SHIPYARD_GIT_USERNAME="+job.Repo.Username,
		"SHIPYARD_GIT_TOKEN="+job.Repo.Token,
	)
	git := func(args ...string) (int, error) {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = ws
		cmd.Env = env
		cmd.WaitDelay = 5 * time.Second
		return runCommand(client, serverURL, token, job.ID, st.ID, cmd, secrets)
	}

	gitRef, gitSHA := "", ""
	if job.Run != nil {
		gitRef, gitSHA = job.Run.GitRef, job.Run.GitSHA
	}

	if code, err := git("init", "-q"); code != 0 {
		return code, err
	}
	if code, err := git("remote", "add", "origin", job.Repo.CloneURL); code != 0 {
		return code, err
	}

	fetchedSHA := false
	if gitSHA != "" {
		code, err := git("fetch", "-q", "--depth", "1", "origin", gitSHA)
		switch {
		case code == 0:
			fetchedSHA = true
		case gitRef != "":
			if code, err := git("fetch", "-q", "--depth", "1", "origin", gitRef); code != 0 {
				return code, err
			}
		default:
			return code, err
		}
	} else {
		target := gitRef
		if target == "" {
			target = "HEAD"
		}
		if code, err := git("fetch", "-q", "--depth", "1", "origin", target); code != 0 {
			return code, err
		}
	}

	checkedOut := false
	if gitSHA != "" && !fetchedSHA {
		if code, _ := git("checkout", "-q", gitSHA); code == 0 {
			checkedOut = true
		}
	}
	if !checkedOut {
		if code, err := git("checkout", "-q", "FETCH_HEAD"); code != 0 {
			return code, err
		}
	}

	what := gitRef
	if gitSHA != "" {
		what = gitSHA
		if len(what) > 7 {
			what = what[:7]
		}
	}
	_ = appendLog(client, serverURL, token, job.ID, st.ID, "system", "checked out "+what+" from "+job.Repo.CloneURL, secrets)
	return 0, nil
}

func execBuildImage(ctx context.Context, client *http.Client, serverURL, token, jobID, ws string, st step, secrets []string) (int, error) {
	bk := buildkit.New(os.Getenv("SHIPYARD_BUILDKIT_ADDR"))
	if !bk.Available(ctx) {
		msg := "buildctl not available; install BuildKit or set SHIPYARD_BUILDKIT_ADDR"
		_ = appendLog(client, serverURL, token, jobID, st.ID, "system", msg, secrets)
		return 1, fmt.Errorf("%s", msg)
	}
	tag := envOr("SHIPYARD_IMAGE_TAG", "shipyard.local/app:latest")
	err := bk.Build(ctx, buildkit.BuildRequest{
		ContextDir: envOr("SHIPYARD_BUILD_CONTEXT", ws),
		Dockerfile: envOr("SHIPYARD_DOCKERFILE", "Dockerfile"),
		Tags:       []string{tag},
		Push:       os.Getenv("SHIPYARD_BUILD_PUSH") == "true",
	}, func(stream, line string) {
		_ = appendLog(client, serverURL, token, jobID, st.ID, stream, line, secrets)
	})
	if err != nil {
		_ = appendLog(client, serverURL, token, jobID, st.ID, "stderr", err.Error(), secrets)
		return 1, err
	}
	_ = appendLog(client, serverURL, token, jobID, st.ID, "system", "image build completed: "+tag, secrets)
	return 0, nil
}

func runCommand(client *http.Client, serverURL, token, jobID, stepID string, cmd *exec.Cmd, secrets []string) (int, error) {
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		_ = appendLog(client, serverURL, token, jobID, stepID, "system", err.Error(), secrets)
		return 1, err
	}
	var streams sync.WaitGroup
	streams.Add(2)
	go func() {
		defer streams.Done()
		streamLogs(client, serverURL, token, jobID, stepID, "stdout", stdout, secrets)
	}()
	go func() {
		defer streams.Done()
		streamLogs(client, serverURL, token, jobID, stepID, "stderr", stderr, secrets)
	}()
	streams.Wait()
	err := cmd.Wait()
	if err == nil {
		return 0, nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), err
	}
	return 1, err
}

const maxLogChunk = 64 << 10

func streamLogs(client *http.Client, serverURL, token, jobID, stepID, stream string, r io.Reader, secrets []string) {
	br := bufio.NewReader(r)
	for {
		chunk, err := br.ReadString('\n')
		line := strings.TrimSuffix(chunk, "\n")
		if err == nil || line != "" {
			for len(line) > maxLogChunk {
				_ = appendLog(client, serverURL, token, jobID, stepID, stream, line[:maxLogChunk], secrets)
				line = line[maxLogChunk:]
			}
			_ = appendLog(client, serverURL, token, jobID, stepID, stream, line, secrets)
		}
		if err != nil {
			return
		}
	}
}

func appendLog(client *http.Client, serverURL, token, jobID, stepID, stream, line string, secrets []string) error {
	return postJSON(client, serverURL+"/api/v1/runner/jobs/"+jobID+"/logs", token, map[string]any{
		"step_id": stepID,
		"stream":  stream,
		"line":    maskSecrets(line, secrets),
	})
}

func maskSecrets(line string, secrets []string) string {
	for _, value := range secrets {
		line = strings.ReplaceAll(line, value, "***")
	}
	return line
}

func postJSON(client *http.Client, url, token string, payload any) error {
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func workRoot() string {
	return envOr("SHIPYARD_WORKDIR", filepath.Join(os.TempDir(), "shipyard-runner"))
}

func askpassPath(jobID string) string {
	return filepath.Join(workRoot(), jobID+".askpass")
}

func envOr(k, v string) string {
	if x := os.Getenv(k); x != "" {
		return x
	}
	return v
}

func runnerLabels(raw string) []string {
	out := []string{}
	seen := map[string]struct{}{}
	for _, l := range append(strings.Split(raw, ","), "os:"+runtime.GOOS) {
		l = strings.ToLower(strings.TrimSpace(l))
		if l == "" {
			continue
		}
		if _, dup := seen[l]; dup {
			continue
		}
		seen[l] = struct{}{}
		out = append(out, l)
	}
	return out
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "runner"
	}
	return h
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
