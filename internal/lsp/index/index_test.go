package index

import (
	"context"
	"os"
	"path"
	"strings"
	"testing"
)

func TestIndex_IndexAll(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.md", `---
id: doc-1
aliases: [foo]
---
# Title
`)
	writeFile(t, dir, "sub/b.md", `# Note B
[[a]]
`)

	idx := New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	tests := []struct {
		name string
		fn   func(*testing.T)
	}{
		{"ListPaths_count", func(t *testing.T) {
			if got := len(idx.ListPaths()); got != 2 {
				t.Errorf("ListPaths: want 2, got %d", got)
			}
		}},
		{"GetByPath_a_has_id", func(t *testing.T) {
			doc := idx.GetByPath("a.md")
			if doc == nil {
				t.Fatal("GetByPath(a.md): nil")
			}
			if doc.ID != "doc-1" {
				t.Errorf("doc.ID: want doc-1, got %q", doc.ID)
			}
		}},
		{"GetByID_doc1", func(t *testing.T) {
			if got := idx.GetByID("doc-1"); got != "a.md" {
				t.Errorf("GetByID(doc-1): want a.md, got %q", got)
			}
		}},
		{"GetByPath_sub_b_has_links", func(t *testing.T) {
			doc := idx.GetByPath("sub/b.md")
			if doc == nil {
				t.Fatal("GetByPath(sub/b.md): nil")
			}
			if len(doc.Links) != 1 {
				t.Errorf("doc.Links: want 1, got %d", len(doc.Links))
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestIndex_AddRemoveUpdate(t *testing.T) {
	dir := t.TempDir()
	idx := New(dir, nil, nil)

	tests := []struct {
		name string
		fn   func(*testing.T)
	}{
		{"Add", func(t *testing.T) {
			if err := idx.Add("new.md", []byte(`# New`)); err != nil {
				t.Fatalf("Add: %v", err)
			}
			if idx.GetByPath("new.md") == nil {
				t.Error("Add: doc not found")
			}
		}},
		{"Update", func(t *testing.T) {
			if err := idx.Update("new.md", []byte(`---
id: updated
---
# Updated`)); err != nil {
				t.Fatalf("Update: %v", err)
			}
			if got := idx.GetByID("updated"); got != "new.md" {
				t.Errorf("GetByID(updated): want new.md, got %q", got)
			}
		}},
		{"Remove", func(t *testing.T) {
			idx.Remove("new.md")
			if idx.GetByPath("new.md") != nil {
				t.Error("Remove: doc still present")
			}
			if idx.GetByID("updated") != "" {
				t.Error("Remove: id still mapped")
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestIndex_IndexAll_ignore(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.md", `# A`)
	writeFile(t, dir, "templates/foo.md", `# Template`)
	writeFile(t, dir, "private/secret.md", `# Secret`)

	ignore := func(p string) bool {
		return strings.HasPrefix(p, "templates/") || strings.HasPrefix(p, "private/")
	}
	idx := New(dir, nil, ignore)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	paths := idx.ListPaths()
	if len(paths) != 1 {
		t.Errorf("ListPaths: want 1 (a.md only), got %d: %v", len(paths), paths)
	}
	if len(paths) > 0 && paths[0] != "a.md" {
		t.Errorf("ListPaths[0]: want a.md, got %q", paths[0])
	}
}

func TestResolveLinkTargetToPath_BasenameIndex(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "z/target.md", "# nested")
	writeFile(t, dir, "target.md", "# root")
	writeFile(t, dir, "x/another.md", "# another")

	idx := New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	if got := idx.ResolveLinkTargetToPath("target"); got != "target.md" {
		t.Fatalf("ResolveLinkTargetToPath(target): want target.md, got %q", got)
	}
	if got := idx.ResolveLinkTargetToPath("target.md"); got != "target.md" {
		t.Fatalf("ResolveLinkTargetToPath(target.md): want target.md, got %q", got)
	}

	// Remove shortest candidate, then basename fallback should pick the remaining one.
	idx.Remove("target.md")
	if got := idx.ResolveLinkTargetToPath("target"); got != "z/target.md" {
		t.Fatalf("ResolveLinkTargetToPath(target) after remove: want z/target.md, got %q", got)
	}
}

func TestResolveLinkTargetToPath_OpenContentIDBeforeReparse(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "target.md", `# Target`)

	idx := New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	if err := idx.SetContent("target.md", []byte(`---
id: live-id
---
# Target`)); err != nil {
		t.Fatalf("SetContent: %v", err)
	}

	if got := idx.ResolveLinkTargetToPath("live-id"); got != "target.md" {
		t.Fatalf("ResolveLinkTargetToPath(live-id): want target.md, got %q", got)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := path.Join(dir, name)
	if err := os.MkdirAll(path.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
