package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFile_Markdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	content := "# Hello\n\nThis is a paragraph.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != content {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestParseFile_Text(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	content := "plain text\nline two\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != content {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestParseFile_NotFound(t *testing.T) {
	_, err := ParseFile("/nonexistent/path/file.pdf")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestParseFile_UnsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")
	// Write some non-PDF, non-ZIP content.
	if err := os.WriteFile(path, []byte("binary data here"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseFile(path)
	if err == nil {
		t.Error("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("error should mention parse: %v", err)
	}
}

func TestParseFile_EmptyMarkdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}
