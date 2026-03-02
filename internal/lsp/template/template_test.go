package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltinDefault(t *testing.T) {
	content := BuiltinDefault()
	if content == "" {
		t.Fatal("BuiltinDefault returned empty")
	}
	args := NowArgs("Test Note")
	got := Execute(content, args)
	if !strings.Contains(got, "Test Note") {
		t.Errorf("expected content to contain 'Test Note', got %q", got)
	}
	if !strings.Contains(got, "title:") {
		t.Errorf("expected frontmatter with title, got %q", got)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	t.Run("default_not_found", func(t *testing.T) {
		content, err := Load(dir, "default")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if content != "" {
			t.Errorf("expected empty when default not found, got %q", content)
		}
	})
	t.Run("custom_found", func(t *testing.T) {
		want := "# {{title}}\n"
		path := filepath.Join(dir, "daily.md")
		if err := os.WriteFile(path, []byte(want), 0644); err != nil {
			t.Fatal(err)
		}
		got, err := Load(dir, "daily")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestExecute(t *testing.T) {
	content := "Date: {{date}}\nTitle: {{title}}"
	args := NowArgs("Foo")
	got := Execute(content, args)
	if !strings.Contains(got, "Foo") {
		t.Errorf("expected Title Foo, got %q", got)
	}
	if !strings.Contains(got, "202") {
		t.Errorf("expected date with year 202x, got %q", got)
	}
}
