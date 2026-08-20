package pipeline

import (
	"fmt"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

type Document struct {
	Pipeline PipelineMeta      `yaml:"pipeline"`
	On       map[string]any    `yaml:"on"`
	Jobs     map[string]JobSpec `yaml:"jobs"`
}

type PipelineMeta struct {
	Name string `yaml:"name"`
}

type JobSpec struct {
	Needs  []string          `yaml:"needs"`
	Runner RunnerSpec        `yaml:"runner"`
	Image  string            `yaml:"image"`
	Steps  []StepSpec        `yaml:"steps"`
	Env    map[string]string `yaml:"env"`
}

type RunnerSpec struct {
	OS     string   `yaml:"os"`
	Labels []string `yaml:"labels"`
	Docker bool     `yaml:"docker"`
}

type StepSpec struct {
	Name string         `yaml:"name"`
	Uses string         `yaml:"uses"`
	Run  string         `yaml:"run"`
	With map[string]any `yaml:"with"`
}

func Parse(source string) (Document, error) {
	var doc Document
	dec := yaml.NewDecoder(strings.NewReader(source))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("parse shipyard.yml: %w", err)
	}
	if err := doc.Validate(); err != nil {
		return Document{}, err
	}
	return doc, nil
}

func (d Document) Validate() error {
	if len(d.Jobs) == 0 {
		return fmt.Errorf("pipeline must define at least one job")
	}
	for name, job := range d.Jobs {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("job name cannot be empty")
		}
		if len(job.Steps) == 0 {
			return fmt.Errorf("job %q must define at least one step", name)
		}
		for i, step := range job.Steps {
			if strings.TrimSpace(step.Run) == "" && strings.TrimSpace(step.Uses) == "" {
				return fmt.Errorf("job %q step %d must set run or uses", name, i)
			}
		}
		for _, need := range job.Needs {
			if _, ok := d.Jobs[need]; !ok {
				return fmt.Errorf("job %q needs unknown job %q", name, need)
			}
			if need == name {
				return fmt.Errorf("job %q cannot depend on itself", name)
			}
		}
	}
	if err := detectCycle(d.Jobs); err != nil {
		return err
	}
	return nil
}

func detectCycle(jobs map[string]JobSpec) error {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var visit func(string) error
	visit = func(name string) error {
		color[name] = gray
		for _, need := range jobs[name].Needs {
			switch color[need] {
			case gray:
				return fmt.Errorf("dependency cycle involving job %q", name)
			case white:
				if err := visit(need); err != nil {
					return err
				}
			}
		}
		color[name] = black
		return nil
	}
	for name := range jobs {
		if color[name] == white {
			if err := visit(name); err != nil {
				return err
			}
		}
	}
	return nil
}

func (j JobSpec) Labels() []string {
	labels := append([]string{}, j.Runner.Labels...)
	if j.Runner.OS != "" {
		labels = append(labels, "os:"+j.Runner.OS)
	}
	if j.Runner.Docker {
		labels = append(labels, "docker")
	}
	if len(labels) == 0 {
		labels = []string{"linux"}
	}
	for i, l := range labels {
		labels[i] = strings.ToLower(l)
	}
	return labels
}

func (d Document) Matches(eventType, gitRef string) bool {
	if len(d.On) == 0 {
		return true
	}
	switch eventType {
	case "push":
		raw, ok := d.On["push"]
		if !ok {
			return false
		}
		spec, ok := raw.(map[string]any)
		if !ok {
			return true
		}
		patterns, ok := spec["branches"].([]any)
		if !ok {
			return true
		}
		branch := strings.TrimPrefix(gitRef, "refs/heads/")
		if branch == gitRef {
			return false
		}
		for _, p := range patterns {
			pattern, ok := p.(string)
			if !ok {
				continue
			}
			if matchBranch(pattern, branch) {
				return true
			}
		}
		return false
	case "pull_request", "pull_request_sync", "merge_request":
		if _, ok := d.On["pull_request"]; ok {
			return true
		}
		_, ok := d.On["merge_request"]
		return ok
	}
	return true
}

func matchBranch(pattern, branch string) bool {
	if pattern == branch || pattern == "**" {
		return true
	}
	if strings.HasSuffix(pattern, "/**") && strings.HasPrefix(branch, strings.TrimSuffix(pattern, "**")) {
		return true
	}
	ok, err := path.Match(pattern, branch)
	return err == nil && ok
}
