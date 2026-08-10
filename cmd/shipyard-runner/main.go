package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/buildkit"
)

func main() {
	serverURL := envOr("SHIPYARD_URL", "http://127.0.0.1:8080")
	token := os.Getenv("SHIPYARD_RUNNER_TOKEN")
	regToken := os.Getenv("SHIPYARD_REGISTRATION_TOKEN")
	name := envOr("SHIPYARD_RUNNER_NAME", hostname())
	labels := strings.Split(envOr("SHIPYARD_RUNNER_LABELS", "linux"), ",")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client := &http.Client{Timeout: 30 * time.Second}

	if token == "" {
		if regToken == "" {
			fatal("SHIPYARD_RUNNER_TOKEN or SHIPYARD_REGISTRATION_TOKEN required")
		}
		body, _ := json.Marshal(map[string]any{
			"token":  regToken,
			"name":   name,
			"labels": labels,
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
			runJob(client, serverURL, token, job)
		}
	}
}

type leaseJobResp struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	LeaseID string `json:"lease_id"`
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

func runJob(client *http.Client, serverURL, token string, job leaseJobResp) {
	_ = postJSON(client, serverURL+"/api/v1/runner/jobs/"+job.ID+"/start", token, map[string]any{"lease_id": job.LeaseID})
	steps := fetchSteps(client, serverURL, token, job.ID)
	failed := false
	for _, step := range steps {
		_ = postJSON(client, serverURL+"/api/v1/runner/steps/"+step.ID+"/status", token, map[string]any{"status": "running"})
		code, err := execStep(client, serverURL, token, job.ID, step)
		status := "succeeded"
		if err != nil || code != 0 {
			status = "failed"
			failed = true
		}
		_ = postJSON(client, serverURL+"/api/v1/runner/steps/"+step.ID+"/status", token, map[string]any{"status": status, "exit_code": code})
		if failed {
			break
		}
	}
	final := "succeeded"
	if failed {
		final = "failed"
	}
	_ = postJSON(client, serverURL+"/api/v1/runner/jobs/"+job.ID+"/complete", token, map[string]any{
		"lease_id": job.LeaseID,
		"status":   final,
	})
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

func execStep(client *http.Client, serverURL, token, jobID string, st step) (int, error) {
	if st.Run == "" && strings.HasPrefix(st.Uses, "shipyard/build-image") {
		return execBuildImage(client, serverURL, token, jobID, st)
	}
	script := st.Run
	if script == "" {
		script = "echo uses=" + st.Uses
	}
	cmd := exec.Command("bash", "-lc", script)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		_ = appendLog(client, serverURL, token, jobID, st.ID, "system", err.Error())
		return 1, err
	}
	go streamLogs(client, serverURL, token, jobID, st.ID, "stdout", stdout)
	go streamLogs(client, serverURL, token, jobID, st.ID, "stderr", stderr)
	err := cmd.Wait()
	if err == nil {
		return 0, nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), err
	}
	return 1, err
}

func execBuildImage(client *http.Client, serverURL, token, jobID string, st step) (int, error) {
	bk := buildkit.New(os.Getenv("SHIPYARD_BUILDKIT_ADDR"))
	if !bk.Available(context.Background()) {
		msg := "buildctl not available; install BuildKit or set SHIPYARD_BUILDKIT_ADDR"
		_ = appendLog(client, serverURL, token, jobID, st.ID, "system", msg)
		return 1, fmt.Errorf("%s", msg)
	}
	tag := envOr("SHIPYARD_IMAGE_TAG", "shipyard.local/app:latest")
	out, err := bk.Build(context.Background(), buildkit.BuildRequest{
		ContextDir: envOr("SHIPYARD_BUILD_CONTEXT", "."),
		Dockerfile: envOr("SHIPYARD_DOCKERFILE", "Dockerfile"),
		Tags:       []string{tag},
		Push:       os.Getenv("SHIPYARD_BUILD_PUSH") == "true",
	})
	if out != "" {
		_ = appendLog(client, serverURL, token, jobID, st.ID, "stdout", out)
	}
	if err != nil {
		_ = appendLog(client, serverURL, token, jobID, st.ID, "stderr", err.Error())
		return 1, err
	}
	_ = appendLog(client, serverURL, token, jobID, st.ID, "system", "image build completed: "+tag)
	return 0, nil
}

func streamLogs(client *http.Client, serverURL, token, jobID, stepID, stream string, r io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			for _, line := range strings.Split(string(buf[:n]), "\n") {
				if line == "" {
					continue
				}
				_ = appendLog(client, serverURL, token, jobID, stepID, stream, line)
			}
		}
		if err != nil {
			return
		}
	}
}

func appendLog(client *http.Client, serverURL, token, jobID, stepID, stream, line string) error {
	return postJSON(client, serverURL+"/api/v1/runner/jobs/"+jobID+"/logs", token, map[string]any{
		"step_id": stepID,
		"stream":  stream,
		"line":    line,
	})
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

func envOr(k, v string) string {
	if x := os.Getenv(k); x != "" {
		return x
	}
	return v
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
