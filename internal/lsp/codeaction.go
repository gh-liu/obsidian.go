package lsp

import (
	"context"
	"fmt"
	"strings"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/protocol"
)

func ResolveCodeAction(ctx context.Context, idx *index.Index, relPath, encoding string, params *protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	if idx == nil || params == nil {
		return nil, nil
	}
	actions := make([]protocol.CodeAction, 0)
	seen := make(map[string]struct{})
	for _, d := range params.Context.Diagnostics {
		if fmt.Sprint(d.Code) != diagCodeBrokenLink {
			continue
		}
		target := extractBrokenLinkTarget(d)
		if target == "" {
			continue
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}

		diag := d
		actions = append(actions, protocol.CodeAction{
			Title:       fmt.Sprintf("Create note '%s'", target),
			Kind:        protocol.QuickFix,
			Diagnostics: []protocol.Diagnostic{diag},
			Command: &protocol.Command{
				Title:     "Create note",
				Command:   cmdCreateNote,
				Arguments: []any{target},
			},
		})
	}
	return actions, nil
}

func extractBrokenLinkTarget(d protocol.Diagnostic) string {
	if m, ok := d.Data.(map[string]any); ok {
		if s, ok := m["target"].(string); ok {
			return strings.TrimSpace(s)
		}
	}
	if x, ok := d.Data.(brokenLinkData); ok {
		return strings.TrimSpace(x.Target)
	}
	const prefix = "Unresolved wikilink target:"
	msg := strings.TrimSpace(d.Message)
	if !strings.HasPrefix(msg, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(msg, prefix))
}
