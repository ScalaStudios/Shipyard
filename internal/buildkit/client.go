package buildkit

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
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

func (c *Client) Build(ctx context.Context, req BuildRequest, onLine func(stream, line string)) error {
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
	cmd.WaitDelay = 5 * time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	var streams sync.WaitGroup
	streams.Add(2)
	go func() {
		defer streams.Done()
		forwardLines(stdout, "stdout", onLine)
	}()
	go func() {
		defer streams.Done()
		forwardLines(stderr, "stderr", onLine)
	}()
	streams.Wait()
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("buildctl failed: %w", err)
	}
	return nil
}

func forwardLines(r io.Reader, stream string, onLine func(stream, line string)) {
	br := bufio.NewReader(r)
	for {
		chunk, err := br.ReadString('\n')
		line := strings.TrimSuffix(chunk, "\n")
		if line != "" && onLine != nil {
			onLine(stream, line)
		}
		if err != nil {
			return
		}
	}
}
