package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gh-liu/obsidian.go/internal/lsp/completion"
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

func TestFormattingUpdatesConfiguredExistingFrontmatterFieldOnly(t *testing.T) {
	h, _, err := NewHandler(context.Background(), nil, nil, nil)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	h.applySettings(map[string]any{
		"format": map[string]any{
			"frontmatter": map[string]any{
				"updatedAt": `{{ now | formatTime "2006-01-02" }}`,
				"missing":   `{{ now | formatTime "2006-01-02" }}`,
			},
		},
	})
	idx := testIndex(t)
	h.index = idx
	content := "---\ntitle: Existing\nupdatedAt: old\n---\n# Body\n"
	if err := idx.SetContent("format_test.md", []byte(content)); err != nil {
		t.Fatalf("SetContent: %v", err)
	}
	defer idx.ClearContent("format_test.md")

	edits, err := h.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.DocumentURI("file://" + filepath.Join(idx.Root(), "format_test.md")),
		},
	})
	if err != nil {
		t.Fatalf("Formatting: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("got %d edits, want 1", len(edits))
	}
	if !strings.Contains(edits[0].NewText, "updatedAt: ") || strings.Contains(edits[0].NewText, "updatedAt: old") {
		t.Fatalf("updatedAt was not replaced:\n%s", edits[0].NewText)
	}
	if strings.Contains(edits[0].NewText, "missing:") {
		t.Fatalf("missing field was added:\n%s", edits[0].NewText)
	}
	if !strings.Contains(edits[0].NewText, "title: Existing") || !strings.Contains(edits[0].NewText, "# Body") {
		t.Fatalf("unrelated content was not preserved:\n%s", edits[0].NewText)
	}
}

func TestCompletionInsertText(t *testing.T) {
	idx := testIndex(t)

	// notes/aaa.md has id=test-aaa, title="Note AAA"
	// Completion [[ test-aaa ]] should produce [[test-aaa|Note AAA]] format
	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(filepath.Join(idx.Root(), "notes/bbb.md")),
			},
			Position: protocol.Position{Line: 0, Character: 0},
		},
	}

	// Open bbb.md with [[test-aa typed at a known line
	content := "# Test\n\n[[test-aa"
	idx.SetContent("notes/bbb.md", []byte(content))
	idx.FlushReparse("notes/bbb.md")

	// Position at end of [[test-aa (line is 9 characters: [[test-aa)
	params.Position = protocol.Position{Line: 2, Character: 9}
	list, err := completion.ResolveCompletion(context.Background(), idx, "notes/bbb.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	if list == nil || len(list.Items) == 0 {
		t.Fatal("no completion items")
	}

	// Find the test-aaa item
	var found bool
	for _, item := range list.Items {
		t.Logf("  completion: label=%q insertText=%q detail=%q", item.Label, item.InsertText, item.Detail)
		if item.InsertText == "test-aaa|Note AAA" {
			found = true
		}
	}
	if !found {
		t.Error("completion for test-aaa should produce [[test-aaa|Note AAA]]")
	}

	idx.ClearContent("notes/bbb.md")
}

func TestEmbedImageCompletion(t *testing.T) {
	idx := testIndex(t)
	imagePath := filepath.Join(idx.Root(), "assets", "cover.png")
	if err := os.MkdirAll(filepath.Dir(imagePath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	defer os.Remove(imagePath)

	content := "# Test\n\n![[cov"
	if err := idx.SetContent("notes/bbb.md", []byte(content)); err != nil {
		t.Fatalf("SetContent: %v", err)
	}
	idx.FlushReparse("notes/bbb.md")
	defer idx.ClearContent("notes/bbb.md")

	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(filepath.Join(idx.Root(), "notes/bbb.md")),
			},
			Position: protocol.Position{Line: 2, Character: 6},
		},
	}
	list, err := completion.ResolveCompletion(context.Background(), idx, "notes/bbb.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	if list == nil {
		t.Fatal("completion list is nil")
	}
	for _, item := range list.Items {
		if item.InsertText == "assets/cover.png" {
			return
		}
	}
	t.Fatalf("image completion missing assets/cover.png: %#v", list.Items)
}

func TestEmbedImageCompletionRespectsImagePaths(t *testing.T) {
	idx := testIndex(t)
	assetImage := filepath.Join(idx.Root(), "assets", "scoped.png")
	otherImage := filepath.Join(idx.Root(), "other", "scoped.png")
	for _, p := range []string{assetImage, otherImage} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(p, []byte("png"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		defer os.Remove(p)
	}

	content := "# Test\n\n![[scoped"
	if err := idx.SetContent("notes/bbb.md", []byte(content)); err != nil {
		t.Fatalf("SetContent: %v", err)
	}
	idx.FlushReparse("notes/bbb.md")
	defer idx.ClearContent("notes/bbb.md")

	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(filepath.Join(idx.Root(), "notes/bbb.md")),
			},
			Position: protocol.Position{Line: 2, Character: 9},
		},
	}
	list, err := completion.ResolveCompletion(context.Background(), idx, "notes/bbb.md", "utf-8", params, []string{"assets"})
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	var foundAsset, foundOther bool
	for _, item := range list.Items {
		foundAsset = foundAsset || item.InsertText == "assets/scoped.png"
		foundOther = foundOther || item.InsertText == "other/scoped.png"
	}
	if !foundAsset || foundOther {
		t.Fatalf("imagePaths should include only assets image; foundAsset=%v foundOther=%v items=%#v", foundAsset, foundOther, list.Items)
	}
}
