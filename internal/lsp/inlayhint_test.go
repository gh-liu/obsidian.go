package lsp

import (
	"context"
	"testing"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/protocol"
)

func TestResolveInlayHint_WikiLinks(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "docs/target.md", `---
title: Target Title
---
# Intro

target block ^ref-block
`)
	writeRefFile(t, dir, "ref.md", `---
id: note-id
title: Reference Title
---
# Ref
`)
	writeRefFile(t, dir, "source.md", `---
title: Source Title
---
# Source

See [[target]] [[note-id]] [[#source]] [[target#Intro]] [[target#^ref-block]] [[missing]]
## Source
`)

	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	params := &InlayHintParams{
		TextDocument: protocol.TextDocumentIdentifier{},
		Range: protocol.Range{
			Start: protocol.Position{Line: 5, Character: 0},
			End:   protocol.Position{Line: 5, Character: 200},
		},
	}
	hints, err := ResolveInlayHint(context.Background(), idx, "source.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveInlayHint: %v", err)
	}
	if len(hints) != 5 {
		t.Fatalf("want 5 hints, got %d", len(hints))
	}

	wantLabels := []string{
		"-> Target Title",
		"-> Reference Title",
		"-> Source Title#Source",
		"-> Target Title#Intro",
		"-> Target Title#^ref-block",
	}
	for i, want := range wantLabels {
		if got := hints[i].Label; got != want {
			t.Fatalf("hint %d label: want %q, got %#v", i, want, got)
		}
		if !hints[i].PaddingLeft {
			t.Fatalf("hint %d: want PaddingLeft=true", i)
		}
	}
}

func TestResolveInlayHint_RespectsRange(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "target.md", "# Target\n")
	writeRefFile(t, dir, "source.md", "[[target]]\n[[target]]\n")

	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	params := &InlayHintParams{
		Range: protocol.Range{
			Start: protocol.Position{Line: 1, Character: 0},
			End:   protocol.Position{Line: 1, Character: 20},
		},
	}
	hints, err := ResolveInlayHint(context.Background(), idx, "source.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveInlayHint: %v", err)
	}
	if len(hints) != 1 {
		t.Fatalf("want 1 hint, got %d", len(hints))
	}
	if hints[0].Position.Line != 1 {
		t.Fatalf("want hint on line 1, got %d", hints[0].Position.Line)
	}
}

func TestMarshalInitializeResultWithInlayHint(t *testing.T) {
	raw, err := marshalInitializeResultWithInlayHint(&protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			DefinitionProvider: true,
		},
		ServerInfo: &protocol.ServerInfo{Name: "obsidian-lsp"},
	})
	if err != nil {
		t.Fatalf("marshalInitializeResultWithInlayHint: %v", err)
	}

	caps, ok := raw["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("want capabilities map, got %#v", raw["capabilities"])
	}
	if got, ok := caps["inlayHintProvider"].(bool); !ok || !got {
		t.Fatalf("want inlayHintProvider=true, got %#v", caps["inlayHintProvider"])
	}
}
