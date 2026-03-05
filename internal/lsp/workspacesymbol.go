package lsp

import (
	"context"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/internal/lsp/position"
	"github.com/gh-liu/obsidian.go/parse"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

const maxWorkspaceSymbols = 200

func ResolveWorkspaceSymbol(ctx context.Context, idx *index.Index, encoding string, params *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	if idx == nil || params == nil {
		return nil, nil
	}
	tags, titleFilter := parseWorkspaceSymbolQuery(params.Query)
	enc := position.Encoder{Encoding: encoding}

	out := make([]protocol.SymbolInformation, 0, maxWorkspaceSymbols)
	for _, entry := range idx.SnapshotPaths() {
		if len(out) >= maxWorkspaceSymbols {
			break
		}
		p := entry.Path
		doc := entry.Doc
		if doc == nil {
			continue
		}
		if !matchesAllTags(doc, tags) {
			continue
		}
		base := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		noteMatched := titleFilter == "" || containsFold(base, titleFilter)
		uri := uri.File(filepath.Join(idx.Root(), p))
		if noteMatched {
			out = append(out, protocol.SymbolInformation{
				Name:     base,
				Kind:     protocol.SymbolKindFile,
				Location: protocol.Location{URI: uri, Range: protocol.Range{}},
			})
			if len(out) >= maxWorkspaceSymbols {
				break
			}
		}
		if titleFilter == "" {
			continue
		}
		for _, h := range doc.Headings {
			if h == nil || !containsFold(h.Text, titleFilter) {
				continue
			}
			out = append(out, protocol.SymbolInformation{
				Name:          h.Text,
				Kind:          protocol.SymbolKindModule,
				ContainerName: base,
				Location: protocol.Location{
					URI:   uri,
					Range: rangeToProtocol(idx, p, h.Range, enc),
				},
			})
			if len(out) >= maxWorkspaceSymbols {
				break
			}
		}
	}
	return out, nil
}

func parseWorkspaceSymbolQuery(query string) ([]string, string) {
	q := strings.TrimSpace(query)
	if q == "" || !strings.HasPrefix(q, "#") {
		return nil, q
	}
	rest := strings.TrimSpace(strings.TrimPrefix(q, "#"))
	if rest == "" {
		return nil, ""
	}
	splitAt := strings.IndexFunc(rest, unicode.IsSpace)
	tagsPart := rest
	title := ""
	if splitAt >= 0 {
		tagsPart = rest[:splitAt]
		title = strings.TrimSpace(rest[splitAt:])
	}
	parts := strings.Split(tagsPart, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		tag := normalizeTag(p)
		if tag == "" {
			continue
		}
		tags = append(tags, tag)
	}
	return tags, title
}

func matchesAllTags(doc *parse.Doc, queryTags []string) bool {
	if len(queryTags) == 0 {
		return true
	}
	if doc == nil || len(doc.Tags) == 0 {
		return false
	}
	available := make(map[string]struct{}, len(doc.Tags))
	for _, t := range doc.Tags {
		n := normalizeTag(t)
		if n == "" {
			continue
		}
		available[n] = struct{}{}
	}
	for _, q := range queryTags {
		if _, ok := available[normalizeTag(q)]; !ok {
			return false
		}
	}
	return true
}

func normalizeTag(s string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(s, "#")))
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
