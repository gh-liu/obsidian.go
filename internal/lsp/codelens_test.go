package lsp

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func TestResolveCodeLens_HeadingReferences(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "a.md", `# A
## Intro
## Intro
## Lonely
`)
	writeRefFile(t, dir, "b.md", `[[a#intro]]
[[a#intro-1]]
`)
	writeRefFile(t, dir, "c.md", `[[a#intro-1]]
`)

	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	params := &protocol.CodeLensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "a.md"))},
	}
	lenses, err := ResolveCodeLens(context.Background(), idx, "a.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveCodeLens: %v", err)
	}
	if len(lenses) != 2 {
		t.Fatalf("want 2 heading lenses, got %d", len(lenses))
	}
	if lenses[0].Command == nil || lenses[0].Command.Title != "1 reference" {
		t.Fatalf("want first lens title '1 reference', got %+v", lenses[0].Command)
	}
	if lenses[0].Command.Command != cmdShowReferences {
		t.Fatalf("want first lens command %q, got %q", cmdShowReferences, lenses[0].Command.Command)
	}
	if lenses[1].Command == nil || lenses[1].Command.Title != "2 references" {
		t.Fatalf("want second lens title '2 references', got %+v", lenses[1].Command)
	}
	if lenses[0].Range.Start.Line != 1 || lenses[1].Range.Start.Line != 2 {
		t.Fatalf("want lenses on lines 1 and 2, got %d and %d", lenses[0].Range.Start.Line, lenses[1].Range.Start.Line)
	}
}

func TestResolveCodeLens_FrontmatterIDReferences(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "a.md", `---
id: note-a
---
# A
## Intro
`)
	writeRefFile(t, dir, "b.md", `[[note-a]]
[[a#intro]]
`)

	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	params := &protocol.CodeLensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "a.md"))},
	}
	lenses, err := ResolveCodeLens(context.Background(), idx, "a.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveCodeLens: %v", err)
	}
	if len(lenses) != 2 {
		t.Fatalf("want 2 lenses (id + heading), got %d", len(lenses))
	}
	if lenses[0].Range.Start.Line != 1 {
		t.Fatalf("want id lens on line 1, got %d", lenses[0].Range.Start.Line)
	}
	if lenses[0].Command == nil || lenses[0].Command.Title != "2 references" {
		t.Fatalf("want id lens title '2 references', got %+v", lenses[0].Command)
	}
}

func TestHandlerInitialize_AdvertisesCodeLens(t *testing.T) {
	h, _, err := NewHandler(context.Background(), nil, nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	result, err := h.Initialize(context.Background(), &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if result.Capabilities.CodeLensProvider == nil {
		t.Fatal("want CodeLensProvider advertised")
	}
	if result.Capabilities.ExecuteCommandProvider == nil {
		t.Fatal("want ExecuteCommandProvider advertised")
	}
	found := false
	for _, cmd := range result.Capabilities.ExecuteCommandProvider.Commands {
		if cmd == cmdShowReferences {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("want execute command list to include %q", cmdShowReferences)
	}
}
