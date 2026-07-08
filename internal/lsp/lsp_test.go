package lsp

import (
	"context"
	"log/slog"
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

func TestInitializeAdvertisesWorkspaceSymbols(t *testing.T) {
	h, _, err := NewHandler(context.Background(), nil, nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	result, err := h.Initialize(context.Background(), &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if result == nil || result.Capabilities.WorkspaceSymbolProvider == nil {
		t.Fatal("WorkspaceSymbolProvider is not advertised")
	}
	if got, ok := result.Capabilities.WorkspaceSymbolProvider.(bool); !ok || !got {
		t.Fatalf("WorkspaceSymbolProvider = %#v, want true", result.Capabilities.WorkspaceSymbolProvider)
	}
}

func TestInitializeAdvertisesHover(t *testing.T) {
	h, _, err := NewHandler(context.Background(), nil, nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	result, err := h.Initialize(context.Background(), &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if got, ok := result.Capabilities.HoverProvider.(bool); !ok || !got {
		t.Fatalf("HoverProvider = %#v, want true", result.Capabilities.HoverProvider)
	}
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

func TestHoverWikiLink(t *testing.T) {
	idx := testIndex(t)
	doc := idx.GetByPath("notes/bbb.md")
	if doc == nil {
		t.Fatal("bbb.md not found")
	}

	var linkLine int
	var linkChar int
	for _, l := range doc.Links {
		if l.Target == "test-aaa" {
			linkLine = l.Range.Start.Line
			linkChar = l.Range.Start.Character + 3
			break
		}
	}
	if linkLine == 0 && linkChar == 0 {
		t.Fatal("link to test-aaa not found in bbb.md")
	}

	hover, err := ResolveHover(context.Background(), idx, "notes/bbb.md", "utf-8", &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(filepath.Join(idx.Root(), "notes/bbb.md")),
			},
			Position: protocol.Position{Line: uint32(linkLine), Character: uint32(linkChar)},
		},
	})
	if err != nil {
		t.Fatalf("ResolveHover: %v", err)
	}
	if hover == nil {
		t.Fatal("got nil hover")
	}
	if hover.Contents.Kind != protocol.Markdown {
		t.Fatalf("hover kind = %q, want markdown", hover.Contents.Kind)
	}
	for _, want := range []string{"**Note AAA**", "`notes/aaa.md`", "# Note AAA"} {
		if !strings.Contains(hover.Contents.Value, want) {
			t.Fatalf("hover content %q does not contain %q", hover.Contents.Value, want)
		}
	}
	if hover.Range == nil {
		t.Fatal("hover range is nil")
	}
}

func TestWorkspaceSymbolNoteTitlesWithTagFilters(t *testing.T) {
	idx := testIndex(t)

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"title", "aaa", []string{"Note AAA"}},
		{"tag", "#tag-b", []string{"Note BBB"}},
		{"title and tag", "note #tag-c", []string{"Note CCC"}},
		{"tag mismatch", "aaa #tag-b", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveWorkspaceSymbol(context.Background(), idx, "utf-8", &protocol.WorkspaceSymbolParams{Query: tt.query})
			if err != nil {
				t.Fatalf("ResolveWorkspaceSymbol: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d symbols, want %d: %#v", len(got), len(tt.want), got)
			}
			for i, want := range tt.want {
				if got[i].Name != want {
					t.Errorf("symbol[%d].Name = %q, want %q", i, got[i].Name, want)
				}
				if got[i].Kind != protocol.SymbolKindFile {
					t.Errorf("symbol[%d].Kind = %v, want file", i, got[i].Kind)
				}
			}
		})
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

func TestBlockCompletionShowsPreviewWithoutGeneratedID(t *testing.T) {
	idx := testIndex(t)
	content := "# Source\n\nParagraph block content ^abc123\n\nTarget link [[#^"
	if err := idx.SetContent("notes/bbb.md", []byte(content)); err != nil {
		t.Fatalf("SetContent: %v", err)
	}
	idx.FlushReparse("notes/bbb.md")
	defer idx.ClearContent("notes/bbb.md")

	list, err := completion.ResolveCompletion(context.Background(), idx, "notes/bbb.md", "utf-8", &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(filepath.Join(idx.Root(), "notes/bbb.md")),
			},
			Position: protocol.Position{Line: 4, Character: 16},
		},
	})
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	if list == nil {
		t.Fatal("completion list is nil")
	}

	var foundExisting bool
	for _, item := range list.Items {
		if item.Label == "abc123" {
			foundExisting = true
			if item.Detail != "Paragraph block content" {
				t.Fatalf("existing block detail = %q, want preview", item.Detail)
			}
		}
		if item.Label == "Generate block ID" {
			t.Fatal("wikilink block completion should not generate new block IDs")
		}
	}
	if !foundExisting {
		t.Fatalf("missing existing block completion: %#v", list.Items)
	}
}

func TestBareCaretCompletionGeneratesBlockID(t *testing.T) {
	idx := testIndex(t)
	content := "# Source\n\nStructured block content\n\n^\n"
	if err := idx.SetContent("notes/bbb.md", []byte(content)); err != nil {
		t.Fatalf("SetContent: %v", err)
	}
	idx.FlushReparse("notes/bbb.md")
	defer idx.ClearContent("notes/bbb.md")

	list, err := completion.ResolveCompletion(context.Background(), idx, "notes/bbb.md", "utf-8", &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(filepath.Join(idx.Root(), "notes/bbb.md")),
			},
			Position: protocol.Position{Line: 4, Character: 1},
		},
	})
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	if list == nil || len(list.Items) != 1 {
		t.Fatalf("items = %#v, want one generate item", list)
	}
	item := list.Items[0]
	if item.Label != "Generate block ID" || item.InsertText == "" || strings.Contains(item.InsertText, "^") {
		t.Fatalf("generated item = %#v", item)
	}
	if len(item.AdditionalTextEdits) != 0 {
		t.Fatalf("bare caret generation should replace typed prefix directly: %#v", item.AdditionalTextEdits)
	}
}

func TestInlineCaretCompletionGeneratesBlockID(t *testing.T) {
	idx := testIndex(t)
	content := "# Source\n\nInline block content ^"
	if err := idx.SetContent("notes/bbb.md", []byte(content)); err != nil {
		t.Fatalf("SetContent: %v", err)
	}
	idx.FlushReparse("notes/bbb.md")
	defer idx.ClearContent("notes/bbb.md")

	list, err := completion.ResolveCompletion(context.Background(), idx, "notes/bbb.md", "utf-8", &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(filepath.Join(idx.Root(), "notes/bbb.md")),
			},
			Position: protocol.Position{Line: 2, Character: 22},
		},
	})
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	if list == nil || len(list.Items) != 1 {
		t.Fatalf("items = %#v, want one generate item", list)
	}
	item := list.Items[0]
	if item.Label != "Generate block ID" || item.InsertText == "" || strings.Contains(item.InsertText, "^") {
		t.Fatalf("generated item = %#v", item)
	}
}

func TestDocumentSymbolHeadingTree(t *testing.T) {
	idx := testIndex(t)

	// Test all 6 levels in chain: H1→H2→H3→H4→H5→H6
	// Test skip-level: #### H4-skip directly under H1 (before any H2/H3 appears)
	// Test siblings: H2 and H2b under H1
	content := "" +
		"# H1\n" +
		"#### H4 skip\n" +
		"## H2\n" +
		"### H3\n" +
		"#### H4\n" +
		"##### H5\n" +
		"###### H6\n" +
		"\n" +
		"## H2b\n" +
		"### H3b\n"

	if err := idx.SetContent("heading_test.md", []byte(content)); err != nil {
		t.Fatalf("SetContent: %v", err)
	}
	idx.FlushReparse("heading_test.md")
	defer idx.ClearContent("heading_test.md")

	symbols, err := ResolveDocumentSymbol(context.Background(), idx, "heading_test.md", "utf-8", &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.DocumentURI("file://" + filepath.Join(idx.Root(), "heading_test.md")),
		},
	})
	if err != nil {
		t.Fatalf("ResolveDocumentSymbol: %v", err)
	}

	// Single root: H1
	if len(symbols) != 1 {
		t.Fatalf("root symbols = %d, want 1", len(symbols))
	}
	if symbols[0].Name != "H1" {
		t.Errorf("root[0].Name = %q, want H1", symbols[0].Name)
	}

	// H1 should have 3 children: H4-skip, H2, H2b (in order of appearance)
	if len(symbols[0].Children) != 3 {
		t.Fatalf("H1 children = %d, want 3", len(symbols[0].Children))
		for _, c := range symbols[0].Children {
			t.Logf("  child: %q", c.Name)
		}
	}

	// First child: skip-level #### H4-skip directly under H1
	h4s := symbols[0].Children[0]
	if h4s.Name != "H4 skip" {
		t.Errorf("H1.child[0] = %q, want 'H4 skip'", h4s.Name)
	}
	if len(h4s.Children) != 0 {
		t.Errorf("H4 skip children = %d, want 0", len(h4s.Children))
	}

	// Second child: H2 (chain H2→H3→H4→H5→H6)
	h2 := symbols[0].Children[1]
	if h2.Name != "H2" {
		t.Errorf("H1.child[1] = %q, want H2", h2.Name)
	}
	if len(h2.Children) != 1 || h2.Children[0].Name != "H3" {
		t.Fatalf("H2 child: got %v", h2.Children)
	}

	// H3 → H4 → H5 → H6 chain
	h := h2.Children[0]
	for _, want := range []string{"H4", "H5", "H6"} {
		if len(h.Children) != 1 {
			t.Fatalf("%s children = %d, want 1", h.Name, len(h.Children))
		}
		h = h.Children[0]
		if h.Name != want {
			t.Errorf("expected %q, got %q", want, h.Name)
		}
	}
	if len(h.Children) != 0 {
		t.Errorf("H6 has %d children, want 0", len(h.Children))
	}

	// Third child: H2b → H3b
	h2b := symbols[0].Children[2]
	if h2b.Name != "H2b" {
		t.Errorf("H1.child[2] = %q, want H2b", h2b.Name)
	}
	if len(h2b.Children) != 1 || h2b.Children[0].Name != "H3b" {
		t.Errorf("H2b children = %v, want [H3b]", h2b.Children)
	}

	t.Log("heading tree: all 6 levels + siblings + skip-level work correctly")
}
