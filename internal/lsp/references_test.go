package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func TestResolveReferences_Backlinks(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "a.md", `# Target`)
	writeRefFile(t, dir, "sub/b.md", `# Note B
See [[a]] and [[a.md]].
`)
	writeRefFile(t, dir, "c.md", `# Note C
[[a]]
`)

	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	// Find refs to a.md (ignore position)
	params := &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "a.md"))},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	}
	locs, err := ResolveReferences(context.Background(), idx, "a.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveReferences: %v", err)
	}
	if len(locs) != 3 {
		t.Errorf("want 3 refs (2 in b.md, 1 in c.md), got %d: %v", len(locs), locs)
	}
}

func TestResolveReferences_ToCurrentFile(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "target.md", `# Target
`)
	writeRefFile(t, dir, "ref.md", `[[target]]
`)

	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	params := &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "target.md"))},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	}
	locs, err := ResolveReferences(context.Background(), idx, "target.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveReferences: %v", err)
	}
	if len(locs) != 1 {
		t.Errorf("want 1 ref (ref.md), got %d", len(locs))
	}
}

func TestResolveReferences_NoSelfDeclaration(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "a.md", `# A`)
	writeRefFile(t, dir, "b.md", `[[a]]`)

	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	// IncludeDeclaration is ignored for file-level refs; file itself is not included
	params := &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "a.md"))},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: true},
	}
	locs, err := ResolveReferences(context.Background(), idx, "a.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveReferences: %v", err)
	}
	if len(locs) != 1 {
		t.Errorf("want 1 ref only (no self), got %d", len(locs))
	}
}

func TestResolveReferences_IncludeHeadingLinks(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "a.md", `# A
## Section
`)
	writeRefFile(t, dir, "b.md", `[[a]]
[[a#Section]]
`)

	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	params := &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "a.md"))},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	}
	locs, err := ResolveReferences(context.Background(), idx, "a.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveReferences: %v", err)
	}
	// Both [[a]] and [[a#Section]] count as references to a.md.
	if len(locs) != 2 {
		t.Errorf("want 2 refs ([[a]] and [[a#Section]]), got %d", len(locs))
	}
}

func TestResolveReferences_HeadingBacklinks(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "a.md", `# A
## Go Memory Model
`)
	writeRefFile(t, dir, "b.md", `[[a#go memory model]]
[[#Go Memory Model]]
`)

	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	params := &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "a.md"))},
			Position:     protocol.Position{Line: 1, Character: 5},
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	}
	locs, err := ResolveReferences(context.Background(), idx, "a.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveReferences: %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("want 1 cross-note heading ref, got %d", len(locs))
	}
	if got := filepath.Base(locs[0].URI.Filename()); got != "b.md" {
		t.Fatalf("want ref in b.md, got %s", got)
	}

	if err := idx.SetContent("a.md", []byte(`# A
## Go Memory Model

See [[#go memory model]]
`)); err != nil {
		t.Fatalf("SetContent: %v", err)
	}
	idx.FlushReparse("a.md")

	params.Context.IncludeDeclaration = true
	locs, err = ResolveReferences(context.Background(), idx, "a.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveReferences with declaration: %v", err)
	}
	if len(locs) != 3 {
		t.Fatalf("want 3 refs including declaration, got %d", len(locs))
	}
}

func TestResolveReferences_HeadingBacklinks_DuplicateAnchors(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "a.md", `# A
## Intro
## Intro
`)
	writeRefFile(t, dir, "b.md", `[[a#intro]]
[[a#intro-1]]
`)

	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	params := &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "a.md"))},
			Position:     protocol.Position{Line: 2, Character: 5},
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	}
	locs, err := ResolveReferences(context.Background(), idx, "a.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveReferences: %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("want 1 ref for second duplicate heading, got %d", len(locs))
	}
	if locs[0].Range.Start.Line != 1 {
		t.Fatalf("want second link line 1, got %d", locs[0].Range.Start.Line)
	}
}

func TestResolveReferences_FileNotInIndex(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "a.md", `# A`)
	writeRefFile(t, dir, "ignored/x.md", `# X`)

	idx := index.New(dir, nil, func(p string) bool { return p == "ignored/x.md" })
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	params := &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "ignored/x.md"))},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	}
	locs, err := ResolveReferences(context.Background(), idx, "ignored/x.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveReferences: %v", err)
	}
	if locs != nil {
		t.Errorf("want nil for file not in index, got %d locs", len(locs))
	}
}

func writeRefFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
