package buildkit

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type BuildRequest struct {
	ContextDir string
	Dockerfile string
	Tags       []string
	Target     string
	BuildArgs  map[string]string
	Push       bool
}

type Client struct {
	Buildctl string
	Addr     string
}

func New(addr string) *Client {
	return &Client{Buildctl: "buildctl", Addr: addr}
}

func (c *Client) Available(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, c.Buildctl, "--version")
	return cmd.Run() == nil
}

func (c *Client) Build(ctx context.Context, req BuildRequest) (string, error) {
	if req.Dockerfile == "" {
		req.Dockerfile = "Dockerfile"
	}
	if req.ContextDir == "" {
		req.ContextDir = "."
	}
	args := []string{}
	if c.Addr != "" {
		args = append(args, "--addr", c.Addr)
	}
	args = append(args, "build",
		"--frontend=dockerfile.v0",
		"--local", "context="+req.ContextDir,
		"--local", "dockerfile="+req.ContextDir,
		"--opt", "filename="+req.Dockerfile,
	)
	if req.Target != "" {
		args = append(args, "--opt", "target="+req.Target)
	}
	for k, v := range req.BuildArgs {
		args = append(args, "--opt", "build-arg:"+k+"="+v)
	}
	for _, tag := range req.Tags {
		if req.Push {
			args = append(args, "--output", "type=image,name="+tag+",push=true")
		} else {
			args = append(args, "--output", "type=docker,name="+tag)
		}
	}
	cmd := exec.CommandContext(ctx, c.Buildctl, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("buildctl failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
