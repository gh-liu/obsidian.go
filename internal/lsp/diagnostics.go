package lsp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/internal/lsp/position"
	"github.com/gh-liu/obsidian.go/parse"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

const (
	diagSourceObsidian = "obsidian-ls"
	diagCodeBrokenLink = "broken-link"
)

type brokenLinkData struct {
	Target string `json:"target"`
}

func diagnoseFile(ctx context.Context, conn jsonrpc2.Conn, idx *index.Index, relPath, encoding string, content []byte) {
	if conn == nil || idx == nil || relPath == "" {
		return
	}
	doc, err := parse.Parse(content, relPath)
	if err != nil {
		return
	}
	enc := position.Encoder{Encoding: encoding}
	lines := strings.Split(string(content), "\n")
	diags := make([]protocol.Diagnostic, 0)
	for _, link := range doc.Links {
		if link == nil || link.Target == "" {
			continue
		}
		if idx.ResolveLinkTargetToPath(link.Target) != "" {
			continue
		}
		diags = append(diags, protocol.Diagnostic{
			Range:    parseRangeToProtocol(link.Range, lines, enc),
			Severity: protocol.DiagnosticSeverityWarning,
			Source:   diagSourceObsidian,
			Code:     diagCodeBrokenLink,
			Message:  fmt.Sprintf("Unresolved wikilink target: %s", link.Target),
			Data:     brokenLinkData{Target: link.Target},
		})
	}
	_ = publishDiagnostics(ctx, conn, idx, relPath, diags)
}

func clearDiagnostics(ctx context.Context, conn jsonrpc2.Conn, idx *index.Index, relPath string) {
	if conn == nil || idx == nil || relPath == "" {
		return
	}
	_ = publishDiagnostics(ctx, conn, idx, relPath, []protocol.Diagnostic{})
}

func publishDiagnostics(ctx context.Context, conn jsonrpc2.Conn, idx *index.Index, relPath string, diags []protocol.Diagnostic) error {
	params := protocol.PublishDiagnosticsParams{
		URI:         uri.File(filepath.Join(idx.Root(), relPath)),
		Diagnostics: diags,
	}
	return conn.Notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, params)
}

func parseRangeToProtocol(r parse.Range, lines []string, enc position.Encoder) protocol.Range {
	startChar := enc.ByteToChar(lineAt(lines, r.Start.Line), r.Start.Character)
	endChar := enc.ByteToChar(lineAt(lines, r.End.Line), r.End.Character)
	return protocol.Range{
		Start: protocol.Position{Line: uint32(r.Start.Line), Character: uint32(startChar)},
		End:   protocol.Position{Line: uint32(r.End.Line), Character: uint32(endChar)},
	}
}
