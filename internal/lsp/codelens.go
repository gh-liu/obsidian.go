package lsp

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/internal/lsp/position"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func ResolveCodeLens(ctx context.Context, idx *index.Index, relPath, encoding string, params *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	if idx == nil || params == nil {
		return nil, nil
	}
	doc := idx.GetByPath(relPath)
	if doc == nil {
		return nil, nil
	}
	enc := position.Encoder{Encoding: encoding}
	docURI := uri.File(filepath.Join(idx.Root(), relPath))

	var lenses []protocol.CodeLens
	if doc.IDRange != nil {
		refs := resolveFileReferences(idx, relPath, enc)
		if len(refs) > 0 {
			idRange := rangeToProtocol(idx, relPath, *doc.IDRange, enc)
			lenses = append(lenses, protocol.CodeLens{
				Range: idRange,
				Command: &protocol.Command{
					Title:     referenceTitle(len(refs)),
					Command:   cmdShowReferences,
					Arguments: []any{docURI, idRange.Start, refs},
				},
			})
		}
	}
	for _, heading := range doc.Headings {
		if heading == nil {
			continue
		}
		refs, matched := resolveHeadingReferences(idx, relPath, heading, enc, false)
		if !matched {
			continue
		}
		headingRange := rangeToProtocol(idx, relPath, heading.Range, enc)
		lenses = append(lenses, protocol.CodeLens{
			Range: headingRange,
			Command: &protocol.Command{
				Title:     referenceTitle(len(refs)),
				Command:   cmdShowReferences,
				Arguments: []any{docURI, headingRange.Start, refs},
			},
		})
	}
	return lenses, nil
}

func referenceTitle(n int) string {
	if n == 1 {
		return "1 reference"
	}
	return fmt.Sprintf("%d references", n)
}
