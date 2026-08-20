package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type client struct {
	base  string
	token string
	http  *http.Client
}

func newClient() *client {
	return &client{
		base:  strings.TrimRight(os.Getenv("SHIPYARD_URL"), "/"),
		token: os.Getenv("SHIPYARD_TOKEN"),
		http:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *client) do(method, path string, body any) (json.RawMessage, error) {
	if c.base == "" || c.token == "" {
		return nil, fmt.Errorf("SHIPYARD_URL and SHIPYARD_TOKEN must be set")
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	return json.RawMessage(data), nil
}

type tool struct {
	name        string
	description string
	schema      json.RawMessage
	call        func(c *client, args map[string]any) (any, error)
}

func argString(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func requireArgs(args map[string]any, keys ...string) error {
	for _, k := range keys {
		if argString(args, k) == "" {
			return fmt.Errorf("missing required argument: %s", k)
		}
	}
	return nil
}

func noArgsSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
}

func objectSchema(required []string, props string) json.RawMessage {
	req, _ := json.Marshal(required)
	return json.RawMessage(fmt.Sprintf(`{"type":"object","properties":{%s},"required":%s,"additionalProperties":false}`, props, req))
}

var tools = []tool{
	{
		name:        "shipyard_system_info",
		description: "Get Shipyard instance information.",
		schema:      noArgsSchema(),
		call: func(c *client, args map[string]any) (any, error) {
			return c.do(http.MethodGet, "/api/v1/system/info", nil)
		},
	},
	{
		name:        "shipyard_whoami",
		description: "Get the authenticated user.",
		schema:      noArgsSchema(),
		call: func(c *client, args map[string]any) (any, error) {
			return c.do(http.MethodGet, "/api/v1/me", nil)
		},
	},
	{
		name:        "shipyard_list_orgs",
		description: "List organizations the token can access.",
		schema:      noArgsSchema(),
		call: func(c *client, args map[string]any) (any, error) {
			return c.do(http.MethodGet, "/api/v1/orgs", nil)
		},
	},
	{
		name:        "shipyard_list_projects",
		description: "List projects in an organization.",
		schema:      objectSchema([]string{"orgID"}, `"orgID":{"type":"string"}`),
		call: func(c *client, args map[string]any) (any, error) {
			if err := requireArgs(args, "orgID"); err != nil {
				return nil, err
			}
			return c.do(http.MethodGet, "/api/v1/orgs/"+argString(args, "orgID")+"/projects", nil)
		},
	},
	{
		name:        "shipyard_list_pipelines",
		description: "List pipeline definitions in a project.",
		schema:      objectSchema([]string{"orgID", "projectID"}, `"orgID":{"type":"string"},"projectID":{"type":"string"}`),
		call: func(c *client, args map[string]any) (any, error) {
			if err := requireArgs(args, "orgID", "projectID"); err != nil {
				return nil, err
			}
			return c.do(http.MethodGet, "/api/v1/orgs/"+argString(args, "orgID")+"/projects/"+argString(args, "projectID")+"/pipelines", nil)
		},
	},
	{
		name:        "shipyard_list_runs",
		description: "List runs in a project.",
		schema:      objectSchema([]string{"orgID", "projectID"}, `"orgID":{"type":"string"},"projectID":{"type":"string"}`),
		call: func(c *client, args map[string]any) (any, error) {
			if err := requireArgs(args, "orgID", "projectID"); err != nil {
				return nil, err
			}
			return c.do(http.MethodGet, "/api/v1/orgs/"+argString(args, "orgID")+"/projects/"+argString(args, "projectID")+"/runs", nil)
		},
	},
	{
		name:        "shipyard_get_run",
		description: "Get a run and its jobs.",
		schema:      objectSchema([]string{"orgID", "projectID", "runID"}, `"orgID":{"type":"string"},"projectID":{"type":"string"},"runID":{"type":"string"}`),
		call: func(c *client, args map[string]any) (any, error) {
			if err := requireArgs(args, "orgID", "projectID", "runID"); err != nil {
				return nil, err
			}
			return c.do(http.MethodGet, "/api/v1/orgs/"+argString(args, "orgID")+"/projects/"+argString(args, "projectID")+"/runs/"+argString(args, "runID"), nil)
		},
	},
	{
		name:        "shipyard_start_run",
		description: "Start a run of a pipeline. Optionally set git_ref and git_sha.",
		schema:      objectSchema([]string{"orgID", "projectID", "pipelineID"}, `"orgID":{"type":"string"},"projectID":{"type":"string"},"pipelineID":{"type":"string"},"git_ref":{"type":"string"},"git_sha":{"type":"string"}`),
		call: func(c *client, args map[string]any) (any, error) {
			if err := requireArgs(args, "orgID", "projectID", "pipelineID"); err != nil {
				return nil, err
			}
			var body any
			if ref, sha := argString(args, "git_ref"), argString(args, "git_sha"); ref != "" || sha != "" {
				body = map[string]string{"git_ref": ref, "git_sha": sha}
			}
			path := "/api/v1/orgs/" + argString(args, "orgID") + "/projects/" + argString(args, "projectID") + "/pipelines/" + argString(args, "pipelineID") + "/runs"
			return c.do(http.MethodPost, path, body)
		},
	},
	{
		name:        "shipyard_cancel_run",
		description: "Cancel a run.",
		schema:      objectSchema([]string{"orgID", "projectID", "runID"}, `"orgID":{"type":"string"},"projectID":{"type":"string"},"runID":{"type":"string"}`),
		call: func(c *client, args map[string]any) (any, error) {
			if err := requireArgs(args, "orgID", "projectID", "runID"); err != nil {
				return nil, err
			}
			path := "/api/v1/orgs/" + argString(args, "orgID") + "/projects/" + argString(args, "projectID") + "/runs/" + argString(args, "runID") + "/cancel"
			return c.do(http.MethodPost, path, nil)
		},
	},
	{
		name:        "shipyard_list_runners",
		description: "List runners visible to the token.",
		schema:      noArgsSchema(),
		call: func(c *client, args map[string]any) (any, error) {
			return c.do(http.MethodGet, "/api/v1/runners", nil)
		},
	},
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func main() {
	c := newClient()
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	out := bufio.NewWriter(os.Stdout)
	for in.Scan() {
		line := bytes.TrimSpace(in.Bytes())
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			fmt.Fprintf(os.Stderr, "shipyard-mcp: bad request: %v\n", err)
			continue
		}
		result, rerr := dispatch(c, &req)
		if req.ID == nil {
			continue
		}
		writeResponse(out, req.ID, result, rerr)
	}
	if err := in.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "shipyard-mcp: read error: %v\n", err)
	}
	_ = out.Flush()
}

func dispatch(c *client, req *rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "shipyard", "version": "0.1.0"},
		}, nil
	case "notifications/initialized":
		return nil, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		list := make([]map[string]any, 0, len(tools))
		for _, t := range tools {
			list = append(list, map[string]any{
				"name":        t.name,
				"description": t.description,
				"inputSchema": t.schema,
			})
		}
		return map[string]any{"tools": list}, nil
	case "tools/call":
		return callTool(c, req.Params), nil
	default:
		return nil, &rpcError{Code: -32601, Message: "method not found"}
	}
}

func callTool(c *client, params json.RawMessage) any {
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return toolError(fmt.Sprintf("invalid params: %v", err))
	}
	for _, t := range tools {
		if t.name == p.Name {
			res, err := t.call(c, p.Arguments)
			if err != nil {
				return toolError(err.Error())
			}
			return toolText(res)
		}
	}
	return toolError("unknown tool: " + p.Name)
}

func toolText(res any) map[string]any {
	var text string
	switch v := res.(type) {
	case json.RawMessage:
		var pretty bytes.Buffer
		if json.Indent(&pretty, v, "", "  ") == nil {
			text = pretty.String()
		} else {
			text = string(v)
		}
	default:
		raw, _ := json.MarshalIndent(v, "", "  ")
		text = string(raw)
	}
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

func toolError(msg string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": msg}},
		"isError": true,
	}
}

func writeResponse(out *bufio.Writer, id json.RawMessage, result any, rerr *rpcError) {
	resp := map[string]any{"jsonrpc": "2.0", "id": id}
	if rerr != nil {
		resp["error"] = rerr
	} else {
		resp["result"] = result
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shipyard-mcp: marshal error: %v\n", err)
		return
	}
	out.Write(raw)
	out.WriteByte('\n')
	_ = out.Flush()
}
