package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltinDefault(t *testing.T) {
	content := defaultTemplate
	if content == "" {
		t.Fatal("DefaultTemplate is empty")
	}
	args := NewVars("Test Note")
	tmp := &Template{Content: content}
	got := tmp.Execute(args)
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
		tmp, err := Load(dir, "default")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if tmp.Content == "" {
			t.Errorf("expected DefaultTemplate when default file not found, got empty")
		}
	})
	t.Run("custom_found", func(t *testing.T) {
		want := "# {{title}}\n"
		path := filepath.Join(dir, "daily.md")
		if err := os.WriteFile(path, []byte(want), 0644); err != nil {
			t.Fatal(err)
		}
		tmp, err := Load(dir, "daily")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if tmp.Content != want {
			t.Errorf("got %q, want %q", tmp.Content, want)
		}
	})
}

func TestExecute(t *testing.T) {
	content := "Date: {{date}}\nTitle: {{title}}"
	args := NewVars("Foo")
	tmp := &Template{Content: content}
	got := tmp.Execute(args)
	if !strings.Contains(got, "Foo") {
		t.Errorf("expected Title Foo, got %q", got)
	}
	if !strings.Contains(got, "202") {
		t.Errorf("expected date with year 202x, got %q", got)
	}
}

func TestGenerateID(t *testing.T) {
	id := generateID()
	if id == "" {
		t.Fatal("GenerateID returned empty")
	}
	// Format: timestamp-XXXX (e.g. 1770123038-LOCB)
	parts := strings.Split(id, "-")
	if len(parts) != 2 {
		t.Errorf("expected format timestamp-XXXX, got %q", id)
	}
	if len(parts[1]) != 4 {
		t.Errorf("expected 4-char suffix, got %q", parts[1])
	}
	for _, c := range parts[1] {
		if c < 'A' || c > 'Z' {
			t.Errorf("suffix must be A-Z, got %q", parts[1])
			break
		}
	}
}

func TestExecute_WithID(t *testing.T) {
	content := `---
title: {{title}}
---
# {{title}}`
	args := Vars{Title: "Test", ID: "1770123038-LOCB", Date: "2025-01-01", Time: "12:00"}
	tmp := &Template{Content: content}
	got := tmp.Execute(args)
	if !strings.Contains(got, "id: 1770123038-LOCB") {
		t.Errorf("expected id in frontmatter, got %q", got)
	}
}

func TestExecute_InjectIDWhenMissing(t *testing.T) {
	content := `---
title: {{title}}
---
body`
	args := Vars{Title: "Test", ID: "1770123038-LOCB", Date: "2025-01-01", Time: "12:00"}
	tmp := &Template{Content: content}
	got := tmp.Execute(args)
	if !strings.Contains(got, "id: 1770123038-LOCB") {
		t.Errorf("expected id injected, got %q", got)
	}
	if !strings.Contains(got, "title: Test") {
		t.Errorf("expected title preserved, got %q", got)
	}
}
