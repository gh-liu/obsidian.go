package lsp

import (
	"context"
	"testing"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/protocol"
)

func TestResolveCodeAction_BrokenLinkQuickFix(t *testing.T) {
	idx := index.New(t.TempDir(), nil, nil)
	params := &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///tmp/note.md"},
		Context: protocol.CodeActionContext{
			Diagnostics: []protocol.Diagnostic{
				{
					Code:    diagCodeBrokenLink,
					Message: "Unresolved wikilink target: missing-note",
					Data:    map[string]any{"target": "missing-note"},
				},
			},
		},
	}
	actions, err := ResolveCodeAction(context.Background(), idx, "note.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveCodeAction: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("want 1 action, got %d", len(actions))
	}
	got := actions[0]
	if got.Kind != protocol.QuickFix {
		t.Fatalf("want kind quickfix, got %q", got.Kind)
	}
	if got.Command == nil || got.Command.Command != cmdCreateNote {
		t.Fatalf("want command %q, got %#v", cmdCreateNote, got.Command)
	}
}
