package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type config struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "login":
		cmdLogin(os.Args[2:])
	case "whoami":
		cmdWhoami()
	case "status":
		cmdStatus()
	case "run":
		cmdRun(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `shipyard — Shipyard CLI

Usage:
  shipyard login --url URL --token TOKEN
  shipyard whoami
  shipyard status
  shipyard run --org ORG_ID --project PROJECT_ID --pipeline PIPELINE_ID
`)
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".shipyard.json"
	}
	return filepath.Join(home, ".config", "shipyard", "config.json")
}

func loadConfig() (config, error) {
	raw, err := os.ReadFile(configPath())
	if err != nil {
		return config{}, err
	}
	var c config
	if err := json.Unmarshal(raw, &c); err != nil {
		return config{}, err
	}
	if c.URL == "" {
		c.URL = "http://127.0.0.1:8080"
	}
	return c, nil
}

func saveConfig(c config) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func cmdLogin(args []string) {
	var url, token string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--url":
			i++
			if i < len(args) {
				url = args[i]
			}
		case "--token":
			i++
			if i < len(args) {
				token = args[i]
			}
		}
	}
	if url == "" {
		url = envOr("SHIPYARD_URL", "http://127.0.0.1:8080")
	}
	if token == "" {
		token = os.Getenv("SHIPYARD_TOKEN")
	}
	if token == "" {
		fmt.Fprintln(os.Stderr, "token required (--token or SHIPYARD_TOKEN)")
		os.Exit(1)
	}
	c := config{URL: strings.TrimRight(url, "/"), Token: token}
	if err := saveConfig(c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("logged in")
}

func cmdWhoami() {
	c, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "not logged in; run shipyard login")
		os.Exit(1)
	}
	var out struct {
		User map[string]any `json:"user"`
	}
	if err := apiGet(c, "/api/v1/me", &out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out.User)
}

func cmdStatus() {
	c, err := loadConfig()
	if err != nil {
		c = config{URL: envOr("SHIPYARD_URL", "http://127.0.0.1:8080")}
	}
	resp, err := http.Get(c.URL + "/api/v1/system/info")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	os.Stdout.Write(body)
	if !bytes.HasSuffix(body, []byte("\n")) {
		fmt.Println()
	}
}

func cmdRun(args []string) {
	var org, project, pipeline string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--org":
			i++
			if i < len(args) {
				org = args[i]
			}
		case "--project":
			i++
			if i < len(args) {
				project = args[i]
			}
		case "--pipeline":
			i++
			if i < len(args) {
				pipeline = args[i]
			}
		}
	}
	if org == "" || project == "" || pipeline == "" {
		fmt.Fprintln(os.Stderr, "--org, --project, and --pipeline are required")
		os.Exit(1)
	}
	c, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "not logged in; run shipyard login")
		os.Exit(1)
	}
	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/pipelines/%s/runs", org, project, pipeline)
	var out map[string]any
	if err := apiPost(c, path, map[string]any{}, &out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func apiGet(c config, path string, dst any) error {
	req, err := http.NewRequest(http.MethodGet, c.URL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func apiPost(c config, path string, payload any, dst any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.URL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
