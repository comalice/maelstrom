package tools

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"
)

func TestNewToolRegistry(t *testing.T) {
	r := NewToolRegistry()
	if r == nil {
		t.Fatal("NewToolRegistry returned nil")
	}
	if len(r.tools) != 0 {
		t.Fatal("NewToolRegistry should have empty tools")
	}
}

func TestRegisterGet(t *testing.T) {
	r := NewToolRegistry()
	mockTool := mockTool{name: "test"}
	r.Register(mockTool)
	got := r.Get("test")
	if got == nil {
		t.Fatal("Get should return registered tool")
	}
	if got.Name() != "test" {
		t.Fatal("wrong tool")
	}
}

func TestList(t *testing.T) {
	r := NewToolRegistry()
	mock1 := mockTool{name: "one"}
	mock2 := mockTool{name: "two"}
	r.Register(mock1)
	r.Register(mock2)
	schemas := r.List()
	if len(schemas) != 2 {
		t.Fatal("List should return 2 schemas")
	}
}

func TestInitRegisters10Tools(t *testing.T) {
	r := NewToolRegistry()
	r.Init()
	if len(r.List()) != 10 {
		t.Fatal("Init should register exactly 10 tools")
	}
	names := map[string]bool{}
	for _, s := range r.List() {
		names[s.Name] = true
	}
	expected := []string{"read_file", "write_file", "web_search", "bash_exec", "list_files", "query_database", "send_http_request", "parse_json", "parse_yaml", "parse_xml"}
	for _, e := range expected {
		if !names[e] {
			t.Fatalf("missing tool %q", e)
		}
	}
}

func TestExecuteAllTools(t *testing.T) {
	r := NewToolRegistry()
	r.Init()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "hello world"

	// test read_file (before write)
	_, err := r.Get("read_file").Execute(context.Background(), map[string]any{"file_path": testFile})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Error("read non-existent should fail with not exist")
	}

	// write_file
	res, err := r.Get("write_file").Execute(context.Background(), map[string]any{"file_path": testFile, "content": content})
	if err != nil {
		t.Fatal(err)
	}
	if res != "File written successfully" {
		t.Error("unexpected write result")
	}

	// read_file after write
	resStr, err := r.Get("read_file").Execute(context.Background(), map[string]any{"file_path": testFile})
	if err != nil {
		t.Fatal(err)
	}
	if resStr != content {
		t.Errorf("read expected %q got %q", content, resStr)
	}

	// list_files
	resList, err := r.Get("list_files").Execute(context.Background(), map[string]any{"pattern": "*.txt", "path": tmpDir})
	if err != nil {
		t.Fatal(err)
	}
	list, ok := resList.([]string)
	if !ok || len(list) == 0 || filepath.Base(list[0]) != "test.txt" {
		t.Error("list_files failed")
	}

	// web_search
	resWeb, err := r.Get("web_search").Execute(context.Background(), map[string]any{"query": "test query"})
	if err != nil {
		t.Fatal(err)
	}
	if exp := `stub: web search 'test query'`; resWeb != exp {
		t.Errorf("web_search expected %q got %q", exp, resWeb)
	}

	// bash_exec read-only: ls
	resBash, err := r.Get("bash_exec").Execute(context.Background(), map[string]any{"command": "ls " + tmpDir})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := resBash.(string); !ok || resBash == "" {
		t.Error("bash_exec ls failed")
	}

	// bash_exec timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	_, err = r.Get("bash_exec").Execute(ctx, map[string]any{"command": "sleep 1"})
	if err == nil {
		t.Error("bash_exec timeout should fail")
	}

	// bash_exec write deny
	_, err = r.Get("bash_exec").Execute(context.Background(), map[string]any{"command": "echo > /tmp/deny.txt"})
	if err == nil || err.Error() != "write commands not allowed (read-only)" {
		t.Errorf("bash_exec write should be denied, got %v", err)
	}

	// query_database
	resDB, err := r.Get("query_database").Execute(context.Background(), map[string]any{"query": "SELECT * FROM users"})
	if err != nil {
		t.Fatal(err)
	}
	users, ok := resDB.([]map[string]any)
	if !ok || len(users) != 2 || users[0]["name"] != "Alice" {
		t.Errorf("query_database expected users, got %v", resDB)
	}

	// send_http_request
	resHTTP, err := r.Get("send_http_request").Execute(context.Background(), map[string]any{"url": "https://httpbin.org/get"})
	if err != nil {
		t.Fatal(err)
	}
	httpRes, ok := resHTTP.(map[string]any)
	if !ok {
		t.Fatal("http res not map")
	}
	statusCode, ok := httpRes["status_code"].(float64)
	if !ok || statusCode != 200 {
		t.Errorf("http expected 200, got %v", httpRes)
	}

	// parse_json
	jsonStr := `{"foo": "bar", "num": 42}`
	resJSON, err := r.Get("parse_json").Execute(context.Background(), map[string]any{"json": jsonStr})
	if err != nil {
		t.Fatal(err)
	}
	jsonMap, ok := resJSON.(map[string]any)
	if !ok || jsonMap["foo"] != "bar" || jsonMap["num"] != float64(42) {
		t.Errorf("parse_json expected {\"foo\":\"bar\",\"num\":42}, got %v", resJSON)
	}

	// parse_yaml
	yamlStr := `foo: bar
num: 42.0`
	resYAML, err := r.Get("parse_yaml").Execute(context.Background(), map[string]any{"yaml": yamlStr})
	if err != nil {
		t.Fatal(err)
	}
	yamlMap, ok := resYAML.(map[string]any)
	if !ok || yamlMap["foo"] != "bar" || yamlMap["num"] != float64(42) {
		t.Errorf("parse_yaml expected {\"foo\":\"bar\",\"num\":42}, got %v", resYAML)
	}

	// parse_xml stub
	resXML, err := r.Get("parse_xml").Execute(context.Background(), map[string]any{"xml": "<root>stub</root>"})
	if err != nil {
		t.Fatal(err)
	}
	if resXML != "Parsed XML: <root>stub</root>" {
		t.Errorf("parse_xml expected Parsed XML stub, got %v", resXML)
	}
}

type mockTool struct {
	name string
}

func (m mockTool) Name() string        { return m.name }
func (m mockTool) Description() string { return "" }
func (m mockTool) Schema() ToolSchema  { return ToolSchema{} }
func (m mockTool) Execute(context.Context, map[string]any) (any, error) {
	return nil, nil
}
