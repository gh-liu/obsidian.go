package lsp

import (
	"testing"

	"github.com/gh-liu/obsidian.go/internal/lsp/position"
	"go.lsp.dev/protocol"
)

func TestApplyContentChanges_UTF16EmojiRange(t *testing.T) {
	// UTF-16 offsets for "A😀B": A[0,1), 😀[1,3), B[3,4)
	before := "A😀B"
	changes := []protocol.TextDocumentContentChangeEvent{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 1},
				End:   protocol.Position{Line: 0, Character: 3},
			},
			Text: "X",
		},
	}

	got := applyContentChanges(before, changes, position.Encoder{Encoding: "utf-16"})
	if got != "AXB" {
		t.Fatalf("unexpected content: got %q, want %q", got, "AXB")
	}
}

func TestApplyContentChanges_UTF16ChineseRange(t *testing.T) {
	// UTF-16 offsets for "a中b": a[0,1), 中[1,2), b[2,3)
	before := "a中b"
	changes := []protocol.TextDocumentContentChangeEvent{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 1},
				End:   protocol.Position{Line: 0, Character: 2},
			},
			Text: "x",
		},
	}

	got := applyContentChanges(before, changes, position.Encoder{Encoding: "utf-16"})
	if got != "axb" {
		t.Fatalf("unexpected content: got %q, want %q", got, "axb")
	}
}
