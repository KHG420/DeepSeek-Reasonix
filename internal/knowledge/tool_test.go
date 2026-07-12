package knowledge

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func newTestTool(t *testing.T) *Tool {
	t.Helper()
	s := tempStore(t)
	return NewTool(s)
}

func TestTool_Name(t *testing.T) {
	tool := newTestTool(t)
	if tool.Name() != "knowledge" {
		t.Errorf("Name: got %q, want knowledge", tool.Name())
	}
}

func TestTool_Description(t *testing.T) {
	tool := newTestTool(t)
	if !strings.Contains(tool.Description(), "domain-specific") {
		t.Error("Description should guide when to search the knowledge base")
	}
}

func TestTool_ReadOnly(t *testing.T) {
	tool := newTestTool(t)
	if tool.ReadOnly() {
		t.Error("knowledge tool should not be read-only (upload/remove are writers)")
	}
}

func TestTool_Schema(t *testing.T) {
	tool := newTestTool(t)
	schema := tool.Schema()
	var m map[string]interface{}
	if err := json.Unmarshal(schema, &m); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	props, ok := m["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing properties")
	}
	op, ok := props["operation"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing operation property")
	}
	enum, ok := op["enum"].([]interface{})
	if !ok {
		t.Fatal("operation missing enum")
	}
	if len(enum) != 5 {
		t.Errorf("expected 5 operations, got %d", len(enum))
	}
}

func TestTool_Execute_Search(t *testing.T) {
	tool := newTestTool(t)
	// Populate a document so there is something to search.
	populateDoc(t, tool.store, "doc", []string{
		"Machine learning is a subset of artificial intelligence.",
		"Deep learning uses neural networks with many layers.",
	})

	args := json.RawMessage(`{"operation":"search","query":"neural networks","limit":5}`)
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Deep learning") {
		t.Errorf("search result should contain matching chunk: %s", result)
	}
}

func TestTool_Execute_Read(t *testing.T) {
	tool := newTestTool(t)
	populateDoc(t, tool.store, "doc", []string{"chunk zero", "chunk one"})

	args := json.RawMessage(`{"operation":"read","docSlug":"doc","chunkID":"001"}`)
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if result != "chunk one" {
		t.Errorf("read: got %q, want 'chunk one'", result)
	}
}

func TestTool_Execute_ReadMissingArgs(t *testing.T) {
	tool := newTestTool(t)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"read"}`))
	if err == nil {
		t.Error("expected error for read without docSlug/chunkID")
	}
}

func TestTool_Execute_List(t *testing.T) {
	tool := newTestTool(t)
	populateDoc(t, tool.store, "doc-a", []string{"a"})
	populateDoc(t, tool.store, "doc-b", []string{"b"})

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"list"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "doc-a") || !strings.Contains(result, "doc-b") {
		t.Errorf("list should contain both docs: %s", result)
	}
}

func TestTool_Execute_ListEmpty(t *testing.T) {
	tool := newTestTool(t)
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"list"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "empty") {
		t.Errorf("empty list should say so: %s", result)
	}
}

func TestTool_Execute_UnknownOperation(t *testing.T) {
	tool := newTestTool(t)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"foobar"}`))
	if err == nil {
		t.Error("expected error for unknown operation")
	}
}

func TestTool_Execute_Upload(t *testing.T) {
	tool := newTestTool(t)
	// Create a temp markdown file.
	dir := t.TempDir()
	path := dir + "/upload-me.md"
	longPara := strings.Repeat("Long paragraph with enough characters to avoid short-chunk merging. ", 6)
	content := "# Test\n\n" + longPara + "\n\n" + longPara
	writeTestFile(t, path, content)

	args := json.RawMessage(`{"operation":"upload","filePath":"` + path + `"}`)
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "uploaded") && !strings.Contains(result, "Document") {
		t.Errorf("unexpected upload result: %s", result)
	}
}

func TestTool_Execute_UploadMissingPath(t *testing.T) {
	tool := newTestTool(t)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"upload"}`))
	if err == nil {
		t.Error("expected error for upload without filePath")
	}
}

func TestTool_Execute_Remove(t *testing.T) {
	tool := newTestTool(t)
	populateDoc(t, tool.store, "rm-me", []string{"content"})

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"remove","docSlug":"rm-me"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "removed") {
		t.Errorf("unexpected remove result: %s", result)
	}
	if tool.store.Exists("rm-me") {
		t.Error("document should be gone after remove")
	}
}

func TestTool_Execute_RemoveMissingSlug(t *testing.T) {
	tool := newTestTool(t)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"remove"}`))
	if err == nil {
		t.Error("expected error for remove without docSlug")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
