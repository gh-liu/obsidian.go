package lsp

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/internal/lsp/position"
	"github.com/gh-liu/obsidian.go/parse"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

const diagSourceObsidian = "obsidian-ls"

func diagnoseFile(ctx context.Context, conn jsonrpc2.Conn, idx *index.Index, relPath, encoding string, content []byte) {
	if conn == nil || idx == nil || relPath == "" {
		return
	}
	doc, err := parse.Parse(content, relPath)
	if err != nil {
		return
	}
	enc := position.Encoder{Encoding: encoding}
	var diags []protocol.Diagnostic
	for _, link := range doc.Links {
		if link == nil || link.Kind != parse.LinkWiki || link.Target == "" {
			continue
		}
		if idx.ResolveLinkTargetToPath(link.Target) != "" {
			continue
		}
		diags = append(diags, protocol.Diagnostic{
			Range:    rangeToProtocol(idx, relPath, link.Range, enc),
			Severity: protocol.DiagnosticSeverityWarning,
			Source:   diagSourceObsidian,
			Code:     "broken-link",
			Message:  fmt.Sprintf("Unresolved wikilink target: %s", link.Target),
		})
	}
	_ = publishDiagnostics(ctx, conn, idx, relPath, diags)
}

func clearDiagnostics(ctx context.Context, conn jsonrpc2.Conn, idx *index.Index, relPath string) {
	_ = publishDiagnostics(ctx, conn, idx, relPath, nil)
}

func publishDiagnostics(ctx context.Context, conn jsonrpc2.Conn, idx *index.Index, relPath string, diags []protocol.Diagnostic) error {
	if diags == nil {
		diags = []protocol.Diagnostic{}
	}
	params := protocol.PublishDiagnosticsParams{
		URI:         uri.File(filepath.Join(idx.Root(), relPath)),
		Diagnostics: diags,
	}
	return conn.Notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, params)
}
