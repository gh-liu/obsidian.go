package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/protocol"
)

func testIndex(t *testing.T) *index.Index {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("index", "testdata"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	idx := index.New(root, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	return idx
}

func TestDefinition(t *testing.T) {
	idx := testIndex(t)

	// Test: [[test-aaa]] in notes/bbb.md links to notes/aaa.md
	doc := idx.GetByPath("notes/bbb.md")
	if doc == nil {
		t.Fatal("bbb.md not found")
	}

	// Find the first link to test-aaa
	var linkLine int
	var linkChar int
	for _, l := range doc.Links {
		if l.Target == "test-aaa" {
			linkLine = l.Range.Start.Line
			linkChar = l.Range.Start.Character + 3 // inside the [[...]]
			break
		}
	}
	if linkLine == 0 && linkChar == 0 {
		t.Fatal("link to test-aaa not found in bbb.md")
	}

	params := &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(filepath.Join(idx.Root(), "notes/bbb.md")),
			},
			Position: protocol.Position{
				Line:      uint32(linkLine),
				Character: uint32(linkChar),
			},
		},
	}

	locs, err := ResolveDefinition(context.Background(), idx, "notes/bbb.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveDefinition: %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("got %d locations, want 1", len(locs))
	}
	expectedPath := protocol.DocumentURI("file://" + filepath.Join(idx.Root(), "notes/aaa.md"))
	if string(locs[0].URI) != string(expectedPath) {
		t.Errorf("location URI = %s, want %s", locs[0].URI, expectedPath)
	}
}

func TestDefinitionHeading(t *testing.T) {
	idx := testIndex(t)

	// Test: [[notes/bbb#Section One]] in sub/ccc.md
	doc := idx.GetByPath("sub/ccc.md")
	if doc == nil {
		t.Fatal("ccc.md not found")
	}

	var linkLine int
	var linkChar int
	for _, l := range doc.Links {
		if l.Target == "notes/bbb" && l.Anchor == "Section One" {
			linkLine = l.Range.Start.Line
			linkChar = l.Range.Start.Character + 3
			break
		}
	}
	if linkLine == 0 && linkChar == 0 {
		t.Fatal("link [[notes/bbb#Section One]] not found in ccc.md")
	}

	params := &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(filepath.Join(idx.Root(), "sub/ccc.md")),
			},
			Position: protocol.Position{
				Line:      uint32(linkLine),
				Character: uint32(linkChar),
			},
		},
	}

	locs, err := ResolveDefinition(context.Background(), idx, "sub/ccc.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveDefinition: %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("got %d locations, want 1", len(locs))
	}
	// Should jump to Section One heading (line > 0)
	if locs[0].Range.Start.Line == 0 {
		t.Error("heading location should not be line 0")
	}
}

func TestReferences(t *testing.T) {
	idx := testIndex(t)

	// aaa.md is referenced by bbb.md, ccc.md, and nofm.md
	params := &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(filepath.Join(idx.Root(), "notes/aaa.md")),
			},
			Position: protocol.Position{Line: 0, Character: 0},
		},
		Context: protocol.ReferenceContext{
			IncludeDeclaration: false,
		},
	}

	locs, err := ResolveReferences(context.Background(), idx, "notes/aaa.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveReferences: %v", err)
	}
	if len(locs) < 2 {
		t.Errorf("got %d references, want >= 2", len(locs))
	}
}

func TestDiagnostics(t *testing.T) {
	idx := testIndex(t)

	// Open a file with a broken link
	content := `# Test

[[nonexistent-note]]
[[test-aaa]]
`
	if err := idx.SetContent("diagnostics_test.md", []byte(content)); err != nil {
		t.Fatalf("SetContent: %v", err)
	}
	idx.FlushReparse("diagnostics_test.md")

	// Check the doc has 2 links
	doc := idx.GetByPath("diagnostics_test.md")
	if doc == nil {
		t.Fatal("doc not found")
	}
	if len(doc.Links) != 2 {
		t.Fatalf("links = %d, want 2", len(doc.Links))
	}

	// [[nonexistent-note]] should not resolve
	if p := idx.ResolveLinkTargetToPath("nonexistent-note"); p != "" {
		t.Errorf("nonexistent-note resolved to %s, want empty", p)
	}

	// [[test-aaa]] should resolve
	if p := idx.ResolveLinkTargetToPath("test-aaa"); p == "" {
		t.Error("test-aaa should resolve")
	}

	idx.ClearContent("diagnostics_test.md")
}

func TestHeadingAnchor(t *testing.T) {
	idx := testIndex(t)

	doc := idx.GetByPath("notes/bbb.md")
	if doc == nil {
		t.Fatal("bbb.md not found")
	}

	// Section One heading should have anchor "section-one"
	anchors := headingAnchors(doc)
	found := false
	for i, h := range doc.Headings {
		if h.Text == "Section One" && anchors[i] == "section-one" {
			found = true
		}
	}
	if !found {
		t.Error("heading 'Section One' should have anchor 'section-one'")
	}
}

func TestMain(m *testing.M) {
	// Ensure we run from the package directory
	dir, _ := os.Getwd()
	if filepath.Base(dir) != "lsp" {
		os.Chdir(filepath.Join(dir, "internal", "lsp"))
	}
	os.Exit(m.Run())
}
