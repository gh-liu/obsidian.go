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

// ResolveReferences resolves textDocument/references for file or heading backlinks.
// When the cursor is on a heading, only references to that heading are returned.
// Otherwise it falls back to file-level backlinks for the current document.
func ResolveReferences(ctx context.Context, idx *index.Index, relPath, encoding string, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	if idx == nil || params == nil {
		return nil, nil
	}
	enc := position.Encoder{Encoding: encoding}

	_, doc, lines := sourceContext(idx, relPath)
	if doc == nil {
		return nil, nil
	}
	lineIdx := int(params.Position.Line)
	if lineIdx >= 0 && lineIdx < len(lines) {
		byteOff := enc.CharToByte(lines[lineIdx], int(params.Position.Character))
		if heading := headingAtPosition(doc, lineIdx, byteOff); heading != nil {
			if locs, matched := resolveHeadingReferences(idx, relPath, heading, enc, params.Context.IncludeDeclaration); matched {
				return locs, nil
			}
		}
	}

	var out []protocol.Location
	out = resolveFileReferences(idx, relPath, enc)
	return out, nil
}

func resolveFileReferences(idx *index.Index, relPath string, enc position.Encoder) []protocol.Location {
	if idx == nil {
		return nil
	}
	var out []protocol.Location
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
			out = append(out, protocol.Location{
				URI:   uri.File(filepath.Join(idx.Root(), p)),
				Range: rangeToProtocol(idx, p, link.Range, enc),
			})
		}
	}
	return out
}

func resolveHeadingReferences(idx *index.Index, relPath string, heading *parse.Heading, enc position.Encoder, includeDeclaration bool) ([]protocol.Location, bool) {
	if idx == nil || heading == nil {
		return nil, false
	}
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
		sourcePath := entry.Path
		doc := entry.Doc
		if doc == nil {
			continue
		}
		for _, link := range doc.Links {
			if !linkMatchesHeading(idx, sourcePath, relPath, wantAnchor, link) {
				continue
			}
			out = append(out, protocol.Location{
				URI:   uri.File(filepath.Join(idx.Root(), sourcePath)),
				Range: rangeToProtocol(idx, sourcePath, link.Range, enc),
			})
		}
	}
	return out, len(out) > 0 && (!includeDeclaration || len(out) > 1)
}

func linkMatchesHeading(idx *index.Index, sourcePath, targetPath, wantAnchor string, link *parse.Link) bool {
	if idx == nil || link == nil || link.Anchor == "" || link.BlockRef != "" {
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
