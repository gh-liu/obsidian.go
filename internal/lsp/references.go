package lsp

import (
	"context"
	"path/filepath"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/internal/lsp/position"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// ResolveReferences resolves textDocument/references for file-level backlinks.
// Returns all links across the vault that point to the current document.
// Ignores cursor position; uses the document's path/id to match.
// Heading references are not implemented yet.
func ResolveReferences(ctx context.Context, idx *index.Index, relPath, encoding string, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	if idx == nil || params == nil {
		return nil, nil
	}
	enc := position.Encoder{Encoding: encoding}

	if idx.GetByPath(relPath) == nil {
		return nil, nil
	}

	var out []protocol.Location
	// Skip IncludeDeclaration for file-level refs: the file itself is not a "reference" to itself.
	for _, entry := range idx.SnapshotPaths() {
		p := entry.Path
		d := entry.Doc
		if d == nil {
			continue
		}
		for _, link := range d.Links {
			if link == nil || link.Target == "" {
				continue
			}
			resolved := idx.ResolveLinkTargetToPath(link.Target)
			if resolved != relPath {
				continue
			}
			loc := protocol.Location{
				URI:   uri.File(filepath.Join(idx.Root(), p)),
				Range: rangeToProtocol(idx, p, link.Range, enc),
			}
			out = append(out, loc)
		}
	}
	return out, nil
}
