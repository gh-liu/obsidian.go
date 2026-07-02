package index

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/gh-liu/obsidian.go/parse"
)

func testdataPath(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return abs
}

func TestIndexAll(t *testing.T) {
	idx := New(testdataPath(t), nil, nil)

	err := idx.IndexAll(context.Background())
	if err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	paths := idx.ListPaths()
	if len(paths) != 4 {
		t.Errorf("paths = %d, want 4: %v", len(paths), paths)
	}

	// Verify all files are indexed
	for _, want := range []string{"notes/aaa.md", "notes/bbb.md", "sub/ccc.md", "nofm.md"} {
		if idx.GetByPath(want) == nil {
			t.Errorf("GetByPath(%q) = nil", want)
		}
	}
}

func TestGetByID(t *testing.T) {
	idx := New(testdataPath(t), nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	// Positive cases
	if p := idx.GetByID("test-aaa"); p != "notes/aaa.md" {
		t.Errorf("GetByID(test-aaa) = %q, want notes/aaa.md", p)
	}
	if p := idx.GetByID("test-bbb"); p != "notes/bbb.md" {
		t.Errorf("GetByID(test-bbb) = %q, want notes/bbb.md", p)
	}
	if p := idx.GetByID("test-ccc"); p != "sub/ccc.md" {
		t.Errorf("GetByID(test-ccc) = %q, want sub/ccc.md", p)
	}

	// Negative case
	if p := idx.GetByID("nonexistent"); p != "" {
		t.Errorf("GetByID(nonexistent) = %q, want empty", p)
	}
}

func TestResolveLinkTargetToPath(t *testing.T) {
	idx := New(testdataPath(t), nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	tests := []struct {
		name   string
		target string
		want   string
	}{
		// Level 1: ID lookup
		{"id", "test-aaa", "notes/aaa.md"},
		{"id2", "test-bbb", "notes/bbb.md"},
		// Level 2: exact path
		{"exact path", "notes/aaa.md", "notes/aaa.md"},
		{"exact path sub", "sub/ccc.md", "sub/ccc.md"},
		// Level 3: path without .md suffix
		{"no suffix", "notes/aaa", "notes/aaa.md"},
		// Level 4: basename fallback
		{"basename", "ccc", "sub/ccc.md"},
		{"basename nofm", "nofm", "nofm.md"},
		// Not found
		{"not found", "nonexistent-id-or-path", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := idx.ResolveLinkTargetToPath(tt.target)
			if got != tt.want {
				t.Errorf("ResolveLinkTargetToPath(%q) = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}

func TestAddUpdateRemove(t *testing.T) {
	root := testdataPath(t)
	idx := New(root, nil, nil)

	// Add a new file
	newPath := filepath.Join(root, "newfile.md")
	content := []byte(`---
id: new-id
title: "New File"
---
# New File

Link to [[test-aaa]].
`)
	if err := os.WriteFile(newPath, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	defer os.Remove(newPath)

	if err := idx.Add("newfile.md", content); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if idx.GetByPath("newfile.md") == nil {
		t.Error("GetByPath(newfile.md) = nil after Add")
	}
	if p := idx.GetByID("new-id"); p != "newfile.md" {
		t.Errorf("GetByID(new-id) = %q after Add", p)
	}

	// Update
	content2 := []byte(`---
id: new-id-v2
title: "Updated File"
---
# Updated
`)
	if err := os.WriteFile(newPath, content2, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := idx.Update("newfile.md", content2); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if idx.GetByID("new-id") != "" {
		t.Error("old id still present after id change")
	}
	if p := idx.GetByID("new-id-v2"); p != "newfile.md" {
		t.Errorf("GetByID(new-id-v2) = %q", p)
	}

	// Remove
	os.Remove(newPath)
	idx.Remove("newfile.md")
	if idx.GetByPath("newfile.md") != nil {
		t.Error("GetByPath(newfile.md) not nil after Remove")
	}
	if idx.GetByID("new-id-v2") != "" {
		t.Error("id still present after Remove")
	}
}

func TestOpenFileOverlay(t *testing.T) {
	idx := New(testdataPath(t), nil, nil)

	unsavedContent := []byte(`---
id: unsaved-id
title: "Unsaved"
---
# Unsaved

[[test-aaa]]
`)
	if err := idx.SetContent("unsaved.md", unsavedContent); err != nil {
		t.Fatalf("SetContent: %v", err)
	}
	// Flush debounce so the reparse completes before assertions
	idx.FlushReparse("unsaved.md")

	// Content should be retrievable
	c, err := idx.GetContent("unsaved.md")
	if err != nil {
		t.Fatalf("GetContent: %v", err)
	}
	if c != string(unsavedContent) {
		t.Errorf("GetContent mismatch")
	}

	// Doc should be in index
	doc := idx.GetByPath("unsaved.md")
	if doc == nil {
		t.Fatal("GetByPath(unsaved.md) = nil after SetContent")
	}
	if doc.ID != "unsaved-id" {
		t.Errorf("doc.ID = %q", doc.ID)
	}

	// HasOpenContent should return true
	if !idx.HasOpenContent("unsaved.md") {
		t.Error("HasOpenContent(unsaved.md) = false, want true")
	}

	// Resolve should find unsaved file by ID (Level 5)
	// Need at least one real file indexed for the ID to be searchable via ResolveLinkTargetToPath
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	// Re-set unsaved content after IndexAll overwrote the index
	if err := idx.SetContent("unsaved.md", unsavedContent); err != nil {
		t.Fatalf("SetContent after IndexAll: %v", err)
	}
	idx.FlushReparse("unsaved.md")

	p := idx.ResolveLinkTargetToPath("unsaved-id")
	if p != "unsaved.md" {
		t.Errorf("ResolveLinkTargetToPath(unsaved-id) = %q, want unsaved.md", p)
	}

	// ClearContent removes from overlay
	idx.ClearContent("unsaved.md")
	if idx.HasOpenContent("unsaved.md") {
		t.Error("HasOpenContent still true after ClearContent")
	}
}

func TestSnapshotAndRange(t *testing.T) {
	idx := New(testdataPath(t), nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	// SnapshotPaths
	snapshot := idx.SnapshotPaths()
	if len(snapshot) != 4 {
		t.Errorf("SnapshotPaths len = %d, want 4", len(snapshot))
	}

	// RangePaths
	count := 0
	idx.RangePaths(func(path string, doc *parse.Doc) bool {
		count++
		if doc == nil {
			t.Errorf("nil doc for %s", path)
		}
		return true
	})
	if count != 4 {
		t.Errorf("RangePaths count = %d, want 4", count)
	}

	// RangePaths with early stop
	count = 0
	idx.RangePaths(func(path string, doc *parse.Doc) bool {
		count++
		return false // stop immediately
	})
	if count != 1 {
		t.Errorf("RangePaths early stop count = %d, want 1", count)
	}
}

func TestGetLines(t *testing.T) {
	idx := New(testdataPath(t), nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	lines, err := idx.GetLines("nofm.md")
	if err != nil {
		t.Fatalf("GetLines: %v", err)
	}
	if len(lines) < 2 {
		t.Errorf("GetLines len = %d, want >= 2", len(lines))
	}
	if lines[0] != "# No Frontmatter" {
		t.Errorf("first line = %q", lines[0])
	}
}

func TestIgnoreFunc(t *testing.T) {
	idx := New(testdataPath(t), slog.Default(), func(path string) bool {
		return filepath.Base(path) == "nofm.md"
	})
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	paths := idx.ListPaths()
	if len(paths) != 3 {
		t.Errorf("paths = %d, want 3 (nofm.md ignored): %v", len(paths), paths)
	}
}

func TestBasenameConflict(t *testing.T) {
	// Create two files with the same basename in different directories
	root := testdataPath(t)
	p1 := filepath.Join(root, "notes", "dupe.md")
	p2 := filepath.Join(root, "sub", "dupe.md")

	content := []byte(`---
id: dupe-notes
---
# Dupe Notes
`)
	if err := os.WriteFile(p1, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	defer os.Remove(p1)

	content2 := []byte(`---
id: dupe-sub
---
# Dupe Sub
`)
	if err := os.WriteFile(p2, content2, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	defer os.Remove(p2)

	idx := New(root, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	// Basename lookup should return the shortest path: sub/dupe.md (len 12) vs notes/dupe.md (len 14)
	got := idx.ResolveLinkTargetToPath("dupe")
	if got != "sub/dupe.md" {
		t.Errorf("ResolveLinkTargetToPath(dupe) = %q, want sub/dupe.md (shortest)", got)
	}
}

func TestDocLinks(t *testing.T) {
	idx := New(testdataPath(t), nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	// bbb.md should have links to aaa, ccc, and #section-one
	doc := idx.GetByPath("notes/bbb.md")
	if doc == nil {
		t.Fatal("bbb.md not indexed")
	}
	if len(doc.Links) < 3 {
		t.Errorf("bbb.md links = %d, want >= 3", len(doc.Links))
	}
}

func TestDebounceReparse(t *testing.T) {
	idx := New(testdataPath(t), nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	// Content is immediately available after SetContent
	c1 := []byte(`---
id: debounce-1
title: "First"
---
# First
`)
	if err := idx.SetContent("debounce.md", c1); err != nil {
		t.Fatalf("SetContent: %v", err)
	}

	// Content readable immediately
	got, err := idx.GetContent("debounce.md")
	if err != nil {
		t.Fatalf("GetContent: %v", err)
	}
	if got != string(c1) {
		t.Error("GetContent mismatch immediately after SetContent")
	}

	// Reparse hasn't happened yet (no FlushReparse called)
	// Doc may or may not be updated depending on goroutine scheduling
	// Flush to make deterministic
	idx.FlushReparse("debounce.md")

	doc := idx.GetByPath("debounce.md")
	if doc == nil || doc.ID != "debounce-1" {
		t.Errorf("after flush: doc.ID = %q, want debounce-1", doc.ID)
	}

	// Multiple rapid SetContent calls should only result in one reparse
	c2 := []byte(`---
id: debounce-2
title: "Second"
---
# Second
`)
	c3 := []byte(`---
id: debounce-3
title: "Third"
---
# Third
`)
	idx.SetContent("debounce.md", c2)
	idx.SetContent("debounce.md", c3)
	idx.FlushReparse("debounce.md")

	doc = idx.GetByPath("debounce.md")
	if doc == nil || doc.ID != "debounce-3" {
		t.Errorf("after rapid updates: doc.ID = %q, want debounce-3", doc.ID)
	}

	// ClearContent cancels pending reparse and reverts
	idx.ClearContent("debounce.md")
	if idx.HasOpenContent("debounce.md") {
		t.Error("HasOpenContent true after ClearContent")
	}
	if idx.GetByPath("debounce.md") != nil {
		t.Error("Doc still indexed after ClearContent (file doesn't exist on disk)")
	}
}
