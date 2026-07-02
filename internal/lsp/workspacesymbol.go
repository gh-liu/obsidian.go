package lsp

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

const maxWorkspaceSymbols = 200

// ResolveWorkspaceSymbol searches notes and headings, with optional tag filter.
func ResolveWorkspaceSymbol(ctx context.Context, idx *index.Index, encoding string, params *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	query := strings.ToLower(params.Query)
	tags, keyword := extractTagFilter(query)

	var results []protocol.SymbolInformation

	for _, entry := range idx.SnapshotPaths() {
		if len(results) >= maxWorkspaceSymbols {
			break
		}
		doc := entry.Doc
		if doc == nil {
			continue
		}

		// Tag filter
		if len(tags) > 0 && !hasAnyTag(doc.Tags, tags) {
			continue
		}

		docURI := uri.File(filepath.Join(idx.Root(), entry.Path))

		// Match file name
		name := strings.TrimSuffix(filepath.Base(entry.Path), ".md")
		displayName := name
		if doc.Title != "" {
			displayName = doc.Title
		}
		if keyword == "" || match(name, keyword) || match(displayName, keyword) {
			results = append(results, protocol.SymbolInformation{
				Name:     displayName,
				Kind:     protocol.SymbolKindFile,
				Location: protocol.Location{URI: docURI, Range: zeroRange()},
			})
		}

		// Match headings
		for _, h := range doc.Headings {
			if h == nil {
				continue
			}
			if keyword == "" || match(h.Text, keyword) {
				if len(results) >= maxWorkspaceSymbols {
					break
				}
				results = append(results, protocol.SymbolInformation{
					Name: h.Text,
					Kind: protocol.SymbolKindString,
					Location: protocol.Location{
						URI: docURI,
						Range: protocol.Range{
							Start: protocol.Position{Line: uint32(h.Range.Start.Line), Character: 0},
							End:   protocol.Position{Line: uint32(h.Range.Start.Line), Character: 0},
						},
					},
				})
			}
		}
	}

	return results, nil
}

func extractTagFilter(query string) (tags []string, keyword string) {
	parts := strings.Fields(query)
	rest := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.HasPrefix(p, "#") {
			tag := strings.TrimPrefix(p, "#")
			for _, t := range strings.Split(tag, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
		} else {
			rest = append(rest, p)
		}
	}
	return tags, strings.Join(rest, " ")
}

func hasAnyTag(docTags, filterTags []string) bool {
	for _, ft := range filterTags {
		for _, dt := range docTags {
			if strings.EqualFold(dt, ft) {
				return true
			}
		}
	}
	return false
}

func match(s, keyword string) bool {
	return strings.Contains(strings.ToLower(s), keyword)
}

func zeroRange() protocol.Range {
	return protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 0, Character: 0},
	}
}
