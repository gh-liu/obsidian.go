package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/internal/lsp/position"
	"github.com/gh-liu/obsidian.go/parse"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// InlayHintParams is a local copy of LSP 3.17 textDocument/inlayHint params.
type InlayHintParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	Range        protocol.Range                  `json:"range"`
}

// InlayHint is a local copy of LSP 3.17 inlay hint payload.
type InlayHint struct {
	Position     protocol.Position `json:"position"`
	Label        string            `json:"label"`
	Kind         uint32            `json:"kind,omitempty"`
	Tooltip      string            `json:"tooltip,omitempty"`
	PaddingLeft  *bool             `json:"paddingLeft,omitempty"`
	PaddingRight *bool             `json:"paddingRight,omitempty"`
}

// ResolveInlayHint resolves textDocument/inlayHint for referenced block IDs.
func ResolveInlayHint(ctx context.Context, idx *index.Index, relPath, encoding string, params *InlayHintParams) ([]InlayHint, error) {
	if idx == nil || params == nil {
		return nil, nil
	}
	_, doc, _ := sourceContext(idx, relPath)
	if doc == nil {
		return nil, nil
	}

	enc := position.Encoder{Encoding: encoding}
	var hints []InlayHint
	for _, heading := range doc.Headings {
		if heading == nil || !rangeIntersects(heading.Range, params.Range) {
			continue
		}
		refs := countHeadingReferences(idx, relPath, heading)
		if refs == 0 {
			continue
		}
		hints = append(hints, referenceInlayHint(idx, relPath, heading.Range, enc, refs))
	}
	for _, block := range doc.Blocks {
		if block == nil || !rangeIntersects(block.Range, params.Range) {
			continue
		}
		refs := countBlockReferences(idx, relPath, block.ID)
		if refs == 0 {
			continue
		}
		hints = append(hints, referenceInlayHint(idx, relPath, block.Range, enc, refs))
	}
	return hints, nil
}

func referenceInlayHint(idx *index.Index, relPath string, r parse.Range, enc position.Encoder, refs int) InlayHint {
	return InlayHint{
		Position:     rangeToProtocol(idx, relPath, r, enc).End,
		Label:        fmt.Sprintf("[%d]ref", refs),
		Kind:         1,
		Tooltip:      fmt.Sprintf("Referenced by %d locations", refs),
		PaddingLeft:  boolPtr(true),
		PaddingRight: boolPtr(false),
	}
}

func countHeadingReferences(idx *index.Index, relPath string, heading *parse.Heading) int {
	wantAnchor := headingAnchor(idx.GetByPath(relPath), heading)
	if wantAnchor == "" {
		return 0
	}
	refs := 0
	for _, entry := range idx.SnapshotPaths() {
		for _, link := range entry.Doc.Links {
			if linkMatchesHeading(idx, entry.Path, relPath, wantAnchor, link) {
				refs++
			}
		}
	}
	return refs
}

func countBlockReferences(idx *index.Index, relPath, blockID string) int {
	refs := 0
	for _, entry := range idx.SnapshotPaths() {
		for _, link := range entry.Doc.Links {
			if linkMatchesBlock(idx, entry.Path, relPath, blockID, link) {
				refs++
			}
		}
	}
	return refs
}

func linkMatchesBlock(idx *index.Index, sourcePath, targetPath, blockID string, link *parse.Link) bool {
	if link == nil || link.BlockRef != blockID {
		return false
	}
	resolvedTarget := sourcePath
	if link.Target != "" {
		resolvedTarget = idx.ResolveLinkTargetToPath(link.Target)
	}
	return resolvedTarget == targetPath
}

func rangeIntersects(r parse.Range, want protocol.Range) bool {
	startLine := int(want.Start.Line)
	endLine := int(want.End.Line)
	return r.End.Line >= startLine && r.Start.Line <= endLine
}

func boolPtr(v bool) *bool { return &v }

func (h *Handler) requestInlayHint(ctx context.Context, raw any) (any, error) {
	if h.index == nil {
		return nil, nil
	}
	var params InlayHintParams
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &params); err != nil {
		return nil, err
	}
	rel := uriToRelPath(params.TextDocument.URI, h.index.Root())
	if rel == "" {
		return nil, nil
	}
	return ResolveInlayHint(ctx, h.index, rel, h.positionEncoding, &params)
}

func blockReferenceLocations(idx *index.Index, relPath, blockID string, enc position.Encoder) []protocol.Location {
	var out []protocol.Location
	for _, entry := range idx.SnapshotPaths() {
		for _, link := range entry.Doc.Links {
			if !linkMatchesBlock(idx, entry.Path, relPath, blockID, link) {
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
