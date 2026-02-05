package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"encoding/json"
	"io"
	"net/http"
	yamlv3 "gopkg.in/yaml.v3"
)

// ParamProperty defines a JSON schema property.
var GlobalTools *ToolRegistry

func init() {
	GlobalTools = NewToolRegistry()
	GlobalTools.Init()
}

type ParamProperty struct {
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
}

// ParamSchema defines the input schema for a tool (JSON Schema object).
type ParamSchema struct {
	Type       string            `json:"type"`
	Properties map[string]ParamProperty `json:"properties"`
	Required   []string          `json:"required,omitempty"`
}

// ToolSchema describes a tool for LLM consumption.
type ToolSchema struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema ParamSchema `json:"inputSchema"`
}

// Result from tool execution.
type Result struct {
	Content any `json:"content"`
}

type rateLimiter struct {
	mu         sync.Mutex
	count      int
	windowStart time.Time
	limit      int
}

func (rl *rateLimiter) TryAcquire() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if now.Sub(rl.windowStart) >= time.Minute {
		rl.windowStart = now
		rl.count = 0
	}

	if rl.count >= rl.limit {
		return false
	}

	rl.count++
	return true
}


// Tool is the interface for executable tools.
type Tool interface {
	Name() string
	Description() string
	Schema() ToolSchema
	Execute(ctx context.Context, params map[string]any) (any, error)
}

// ToolRegistry manages a collection of tools.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	rateLimiters sync.Map
}

// NewToolRegistry creates a new ToolRegistry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// Get retrieves a tool by name.
func (r *ToolRegistry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// List returns schemas for all registered tools.
func (r *ToolRegistry) List() []ToolSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()
	schemas := make([]ToolSchema, 0, len(r.tools))
	for _, t := range r.tools {
		schemas = append(schemas, t.Schema())
	}
	return schemas
}

// Init registers the 10 core tools: 5 file/system + 5 data/HTTP.
func (r *ToolRegistry) Init() {
	r.Register(readFileTool{})
	r.Register(writeFileTool{})
	r.Register(webSearchTool{})
	r.Register(bashExecTool{})
	r.Register(listFilesTool{})
	r.Register(queryDatabaseTool{})
	r.Register(sendHTTPRequestTool{})
	r.Register(parseJSONTool{})
	r.Register(parseYAMLTool{})
	r.Register(parseXMLTool{})
}

func (r *ToolRegistry) getRateLimiter(toolName string, limit int) *rateLimiter {
	if limit <= 0 {
		return nil
	}

	key := fmt.Sprintf("%s_%d", toolName, limit)

	nrl := &rateLimiter{
		limit:      limit,
		windowStart: time.Now(),
	}

	if rlI, loaded := r.rateLimiters.LoadOrStore(key, nrl); loaded {
		return rlI.(*rateLimiter)
	}
	return nrl
}
// readFileTool stub using os.ReadFile.
type readFileTool struct{}

func (readFileTool) Name() string {
	return "read_file"
}
func (readFileTool) Description() string {
	return "Read the contents of a file (absolute path)."
}
func (readFileTool) Schema() ToolSchema {
	return ToolSchema{
		Name: "read_file",
		Description: "Read the contents of a file.",
		InputSchema: ParamSchema{
			Type: "object",
			Properties: map[string]ParamProperty{
				"file_path": {Type: "string", Description: "Absolute path to the file."},
			},
			Required: []string{"file_path"},
		},
	}
}

func (t readFileTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	path, ok := params["file_path"].(string)
	if !ok {
		return nil, errors.New("file_path must be string")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return string(data), nil
}

// writeFileTool stub using os.WriteFile.
type writeFileTool struct{}

func (writeFileTool) Name() string {
	return "write_file"
}
func (writeFileTool) Description() string {
	return "Write content to a file (absolute path)."
}
func (writeFileTool) Schema() ToolSchema {
	return ToolSchema{
		Name: "write_file",
		Description: "Write string content to a file.",
		InputSchema: ParamSchema{
			Type: "object",
			Properties: map[string]ParamProperty{
				"file_path": {Type: "string", Description: "Absolute path to the file."},
				"content": {Type: "string", Description: "Content to write."},
			},
			Required: []string{"file_path", "content"},
		},
	}
}

func (t writeFileTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	path, ok := params["file_path"].(string)
	if !ok {
		return nil, errors.New("file_path must be string")
	}
	content, ok := params["content"].(string)
	if !ok {
		return nil, errors.New("content must be string")
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}
	return "File written successfully", nil
}

// webSearchTool stub.
type webSearchTool struct{}

func (webSearchTool) Name() string {
	return "web_search"
}
func (webSearchTool) Description() string {
	return "Stub web search."
}
func (webSearchTool) Schema() ToolSchema {
	return ToolSchema{
		Name: "web_search",
		Description: "Perform a web search (stub).",
		InputSchema: ParamSchema{
			Type: "object",
			Properties: map[string]ParamProperty{
				"query": {Type: "string", Description: "Search query."},
			},
			Required: []string{"query"},
		},
	}
}

func (t webSearchTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	query, ok := params["query"].(string)
	if !ok {
		return nil, errors.New("query must be string")
	}
	return fmt.Sprintf("stub: web search '%s'", query), nil
}

// bashExecTool stub: exec bash with timeout, read-only warn on writes.
type bashExecTool struct{}

func (bashExecTool) Name() string {
	return "bash_exec"
}
func (bashExecTool) Description() string {
	return "Execute bash command (sandboxed via policies, max 5s timeout)."
}
func (bashExecTool) Schema() ToolSchema {
	return ToolSchema{
		Name: "bash_exec",
		Description: "Execute bash command.",
		InputSchema: ParamSchema{
			Type: "object",
			Properties: map[string]ParamProperty{
				"command": {Type: "string", Description: "Bash command."},
				"timeout": {Type: "string", Description: `Timeout e.g. "30s".`},
			},
			Required: []string{"command"},
		},
	}
}

func (t bashExecTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	cmdStr, ok := params["command"].(string)
	if !ok {
		return nil, errors.New("command must be string")
	}
	// Warn if potential write cmds
	if strings.Contains(cmdStr, " >") || strings.Contains(cmdStr, "<") || (strings.Contains(cmdStr, "|") && strings.Contains(cmdStr, "tee")) {
		return nil, errors.New("write commands not allowed (read-only)")
	}
	timeoutStr, _ := params["timeout"].(string)
	timeout := 2 * time.Minute
	if timeoutStr != "" {
		d, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout: %w", err)
		}
		timeout = d
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", cmdStr)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("stderr: %s\nerror: %v", stderr.String(), err), err
	}
	return stdout.String(), nil
}

// listFilesTool using filepath.Glob.
type listFilesTool struct{}

func (listFilesTool) Name() string {
	return "list_files"
}
func (listFilesTool) Description() string {
	return "List files matching glob pattern."
}
func (listFilesTool) Schema() ToolSchema {
	return ToolSchema{
		Name: "list_files",
		Description: "List files by glob pattern.",
		InputSchema: ParamSchema{
			Type: "object",
			Properties: map[string]ParamProperty{
				"pattern": {Type: "string", Description: `Glob pattern e.g. **/*.go`},
				"path": {Type: "string", Description: "Base directory, default cwd."},
			},
			Required: []string{"pattern"},
		},
	}
}

func (t listFilesTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	pattern, ok := params["pattern"].(string)
	if !ok {
		return nil, errors.New("pattern must be string")
	}
	path, _ := params["path"].(string)
	if path == "" {
		path = "."
	}
	matches, err := filepath.Glob(filepath.Join(path, pattern))
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}
	return matches, nil
}

type queryDatabaseTool struct{}

func (queryDatabaseTool) Name() string {
	return "query_database"
}

func (queryDatabaseTool) Description() string {
	return "Query a mock database."
}

func (queryDatabaseTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "query_database",
		Description: "Execute a SQL-like query on a mock database.",
		InputSchema: ParamSchema{
			Type: "object",
			Properties: map[string]ParamProperty{
				"query": {Type: "string", Description: "SQL query string."},
			},
			Required: []string{"query"},
		},
	}
}

func (t queryDatabaseTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	query, ok := params["query"].(string)
	if !ok {
		return nil, errors.New("query must be string")
	}
	lowerQuery := strings.ToLower(query)
	words := strings.Fields(lowerQuery)
	for i := range words {
		if words[i] == "from" && i+1 < len(words) {
			table := strings.TrimSpace(strings.TrimSuffix(words[i+1], ";"))
			if table == "users" {
				return []map[string]any{
					{"id": 1, "name": "Alice"},
					{"id": 2, "name": "Bob"},
				}, nil
			}
		}
	}
	return []map[string]any{}, nil
}

type sendHTTPRequestTool struct{}

func (sendHTTPRequestTool) Name() string {
	return "send_http_request"
}

func (sendHTTPRequestTool) Description() string {
	return "Send an HTTP request using http.Client."
}

func (sendHTTPRequestTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "send_http_request",
		Description: "Send HTTP request to a URL (uses httpbin for tests).",
		InputSchema: ParamSchema{
			Type: "object",
			Properties: map[string]ParamProperty{
				"url":    {Type: "string", Description: "Target URL."},
				"method": {Type: "string", Description: "HTTP method (default: GET)."},
				"body":   {Type: "string", Description: "Request body (JSON)."},
			},
			Required: []string{"url"},
		},
	}
}

func (t sendHTTPRequestTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	url, ok := params["url"].(string)
	if !ok {
		return nil, errors.New("url must be string")
	}
	method, _ := params["method"].(string)
	if method == "" {
		method = "GET"
	}
	bodyStr, _ := params["body"].(string)
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(bodyStr))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	if bodyStr != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "Maelstrom-Tool/1.0")
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return map[string]any{
		"status_code": float64(resp.StatusCode),
		"status":      resp.Status,
		"body":        string(bodyBytes),
	}, nil
}

type parseJSONTool struct{}

func (parseJSONTool) Name() string {
	return "parse_json"
}

func (parseJSONTool) Description() string {
	return "Parse JSON string to object/map."
}

func (parseJSONTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "parse_json",
		Description: "Unmarshal JSON string.",
		InputSchema: ParamSchema{
			Type: "object",
			Properties: map[string]ParamProperty{
				"json": {Type: "string", Description: "JSON string."},
			},
			Required: []string{"json"},
		},
	}
}

func (t parseJSONTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	jsonStr, ok := params["json"].(string)
	if !ok {
		return nil, errors.New("json must be string")
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &out); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	return out, nil
}

type parseYAMLTool struct{}

func (parseYAMLTool) Name() string {
	return "parse_yaml"
}

func (parseYAMLTool) Description() string {
	return "Parse YAML string to object/map."
}

func (parseYAMLTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "parse_yaml",
		Description: "Unmarshal YAML string.",
		InputSchema: ParamSchema{
			Type: "object",
			Properties: map[string]ParamProperty{
				"yaml": {Type: "string", Description: "YAML string."},
			},
			Required: []string{"yaml"},
		},
	}
}

func (t parseYAMLTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	yamlStr, ok := params["yaml"].(string)
	if !ok {
		return nil, errors.New("yaml must be string")
	}
	var out map[string]any
	if err := yamlv3.Unmarshal([]byte(yamlStr), &out); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}
	return out, nil
}

type parseXMLTool struct{}

func (parseXMLTool) Name() string {
	return "parse_xml"
}

func (parseXMLTool) Description() string {
	return "Stub for XML parsing."
}

func (parseXMLTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "parse_xml",
		Description: "Parse XML string (stub).",
		InputSchema: ParamSchema{
			Type: "object",
			Properties: map[string]ParamProperty{
				"xml": {Type: "string", Description: "XML string."},
			},
			Required: []string{"xml"},
		},
	}
}

func (t parseXMLTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	xmlStr, ok := params["xml"].(string)
	if !ok {
		return nil, errors.New("xml must be string")
	}
	data := []byte(xmlStr)
	if len(data) > 100 {
		data = data[:100]
	}
	return fmt.Sprintf("Parsed XML: %s", data), nil
}

func (r *ToolRegistry) EnforcePolicies(toolName string, policies []string, params map[string]any, ctx context.Context) error {
	var allowedSet = map[string]bool{}
	var forbiddenSet = map[string]bool{}
	var rateN int
	var cost float64 = 0.01

	for _, pol := range policies {
		idx := strings.Index(pol, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(pol[:idx])
		val := strings.TrimSpace(pol[idx+1:])

		switch key {
		case "rate_limit":
			if strings.HasSuffix(val, "/min") {
				nStr := strings.TrimSuffix(val, "/min")
				if n, err := strconv.Atoi(strings.TrimSpace(nStr)); err == nil {
					rateN = n
				}
			}
		case "cost":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				cost = f
			}
		case "allowed":
			for _, part := range strings.Split(val, ",") {
				cmd := strings.TrimSpace(part)
				allowedSet[cmd] = true
			}
		case "forbidden":
			for _, part := range strings.Split(val, ",") {
				cmd := strings.TrimSpace(part)
				forbiddenSet[cmd] = true
			}
		}
	}

	// rate limit
	if rateN > 0 {
		rl := r.getRateLimiter(toolName, rateN)
		if !rl.TryAcquire() {
			return fmt.Errorf("rate limit exceeded for tool %q: %d/min", toolName, rateN)
		}
	}

	// cost stub
	fmt.Printf("[TOOL-COST] %s: %.4f\n", toolName, cost)

	// bash_exec sandbox
	if toolName == "bash_exec" {
		cmdStrI, ok := params["command"]
		if !ok {
			return errors.New("bash_exec: missing 'command'")
		}
		cmdStr, ok := cmdStrI.(string)
		if !ok {
			return errors.New("bash_exec: 'command' must be string")
		}
		fields := strings.Fields(cmdStr)
		if len(fields) == 0 {
			return errors.New("bash_exec: empty command")
		}
		cmd := fields[0]

		if len(allowedSet) > 0 {
			if !allowedSet[cmd] {
				return fmt.Errorf("bash_exec command %q not allowed, allowed=%v", cmd, allowedSet)
			}
		}
		for fb := range forbiddenSet {
			if strings.Contains(cmdStr, fb) {
				return fmt.Errorf("bash_exec forbidden %q in command %q", fb, cmdStr)
			}
		}
	}

	return nil
}

func (r *ToolRegistry) Execute(ctx context.Context, toolName string, params map[string]any, policies []string) (any, error) {
	tool := r.Get(toolName)
	if tool == nil {
		return nil, fmt.Errorf("unknown tool %q", toolName)
	}

	if err := r.EnforcePolicies(toolName, policies, params, ctx); err != nil {
		return nil, err
	}

	execCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return tool.Execute(execCtx, params)
}
