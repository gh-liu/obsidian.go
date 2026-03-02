package lsp

import (
	"context"
	"path/filepath"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// ResolveReferences resolves textDocument/references for file-level backlinks.
// Returns all links across the vault that point to the current document.
// Ignores cursor position; uses the document's path/id to match.
// Heading references are not implemented yet.
func ResolveReferences(ctx context.Context, idx *index.Index, root, encoding string, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	if idx == nil || params == nil {
		return nil, nil
	}
	enc := PositionEncoder{encoding: encoding}

	targetPath := referencesTargetPath(idx, root, params)
	if targetPath == "" {
		return nil, nil
	}

	var out []protocol.Location
	// Skip IncludeDeclaration for file-level refs: the file itself is not a "reference" to itself.
	for _, p := range idx.ListPaths() {
		d := idx.GetByPath(p)
		if d == nil {
			continue
		}
		for _, link := range d.Links {
			if link == nil || link.Target == nil {
				continue
			}
			if link.Anchor != nil {
				continue // exclude [[file#heading]], only count file-level refs
			}
			resolved := idx.ResolveLinkTargetToPath(link.Target.ID)
			if resolved != targetPath {
				continue
			}
			loc := protocol.Location{
				URI:   uri.File(filepath.Join(root, p)),
				Range: rangeToProtocol(p, root, link.Range, enc),
			}
			out = append(out, loc)
		}
	}
	return out, nil
}

func referencesTargetPath(idx *index.Index, root string, params *protocol.ReferenceParams) string {
	fullPath := uri.URI(params.TextDocument.URI).Filename()
	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if idx.GetByPath(rel) == nil {
		return ""
	}
	return rel
}
