package lsp

import (
	"context"
	"path/filepath"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/internal/lsp/position"
	"github.com/gh-liu/obsidian.go/parse"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// ResolveReferences resolves textDocument/references for file backlinks or heading references.
func ResolveReferences(ctx context.Context, idx *index.Index, relPath, encoding string, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	if idx == nil || params == nil {
		return nil, nil
	}
	enc := position.Encoder{Encoding: encoding}

	_, doc, lines := sourceContext(idx, relPath)
	if doc == nil {
		return nil, nil
	}

	_ = lines

	// File-level backlinks. Heading references are intentionally not resolved yet.
	return resolveFileReferences(idx, relPath, enc), nil
}

func headingAtPosition(doc *parse.Doc, lineIdx, byteOff int) *parse.Heading {
	for _, h := range doc.Headings {
		if h == nil {
			continue
		}
		if lineIdx >= h.Range.Start.Line && lineIdx <= h.Range.End.Line {
			if lineIdx > h.Range.Start.Line || byteOff >= h.Range.Start.Character {
				return h
			}
		}
	}
	return nil
}

func resolveFileReferences(idx *index.Index, relPath string, enc position.Encoder) []protocol.Location {
	var out []protocol.Location
	for _, entry := range idx.SnapshotPaths() {
		for _, link := range entry.Doc.Links {
			if link == nil || link.Target == "" || link.Kind != parse.LinkWiki {
				continue
			}
			if idx.ResolveLinkTargetToPath(link.Target) != relPath {
				continue
			}
			out = append(out, protocol.Location{
				URI:   uri.File(filepath.Join(idx.Root(), entry.Path)),
				Range: rangeToProtocol(idx, entry.Path, link.Range, enc),
			})
		}
	}
	return out
}

func resolveHeadingReferences(idx *index.Index, relPath string, heading *parse.Heading, enc position.Encoder, includeDeclaration bool) ([]protocol.Location, bool) {
	wantAnchor := headingAnchor(idx.GetByPath(relPath), heading)
	if wantAnchor == "" {
		return nil, false
	}

	var out []protocol.Location
	if includeDeclaration {
		out = append(out, protocol.Location{
			URI:   uri.File(filepath.Join(idx.Root(), relPath)),
			Range: rangeToProtocol(idx, relPath, heading.Range, enc),
		})
	}

	for _, entry := range idx.SnapshotPaths() {
		for _, link := range entry.Doc.Links {
			if !linkMatchesHeading(idx, entry.Path, relPath, wantAnchor, link) {
				continue
			}
			out = append(out, protocol.Location{
				URI:   uri.File(filepath.Join(idx.Root(), entry.Path)),
				Range: rangeToProtocol(idx, entry.Path, link.Range, enc),
			})
		}
	}
	return out, len(out) > 0 && (!includeDeclaration || len(out) > 1)
}

func linkMatchesHeading(idx *index.Index, sourcePath, targetPath, wantAnchor string, link *parse.Link) bool {
	if link == nil || link.Anchor == "" || link.BlockRef != "" {
		return false
	}
	resolvedTarget := sourcePath
	if link.Target != "" {
		resolvedTarget = idx.ResolveLinkTargetToPath(link.Target)
	}
	if resolvedTarget != targetPath {
		return false
	}
	return normalizeHeadingAnchor(link.Anchor) == wantAnchor
}
