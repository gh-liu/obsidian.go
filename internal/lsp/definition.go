package lsp

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/internal/lsp/position"
	"github.com/gh-liu/obsidian.go/parse"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// ResolveDefinition resolves textDocument/definition for Obsidian wiki links.
func ResolveDefinition(ctx context.Context, idx *index.Index, relPath, encoding string, params *protocol.DefinitionParams) ([]protocol.Location, error) {
	if idx == nil || params == nil {
		return nil, nil
	}
	enc := position.Encoder{Encoding: encoding}
	rel, doc, lines := sourceContext(idx, relPath)
	if doc == nil {
		return nil, nil
	}
	lineIdx := int(params.Position.Line)
	if lineIdx < 0 || lineIdx >= len(lines) {
		return nil, nil
	}
	byteOff := enc.CharToByte(lines[lineIdx], int(params.Position.Character))

	link := linkAtPosition(doc, lineIdx, byteOff)
	if link == nil {
		return nil, nil
	}

	var targetPath string
	if link.Target != "" {
		targetPath = idx.ResolveLinkTargetToPath(link.Target)
		if targetPath == "" {
			return nil, nil
		}
	} else {
		targetPath = rel
	}

	var loc protocol.Location
	switch {
	case link.BlockRef != "":
		loc = targetLocationBlock(idx, targetPath, link.BlockRef, enc)
	case link.Anchor != "":
		loc = targetLocation(idx, targetPath, link.Anchor, enc)
	default:
		loc = targetLocation(idx, targetPath, "", enc)
	}
	return []protocol.Location{loc}, nil
}

func sourceContext(idx *index.Index, rel string) (string, *parse.Doc, []string) {
	doc := idx.GetByPath(rel)
	if doc == nil {
		return "", nil, nil
	}
	content, err := idx.GetContent(rel)
	if err != nil {
		return "", nil, nil
	}
	return rel, doc, strings.Split(content, "\n")
}

func linkAtPosition(doc *parse.Doc, lineIdx, byteOff int) *parse.Link {
	for _, link := range doc.Links {
		if link == nil {
			continue
		}
		r := link.Range
		if lineIdx < r.Start.Line || lineIdx > r.End.Line {
			continue
		}
		if lineIdx == r.Start.Line && byteOff < r.Start.Character {
			continue
		}
		if lineIdx == r.End.Line && byteOff >= r.End.Character {
			continue
		}
		return link
	}
	return nil
}

func targetLocation(idx *index.Index, targetPath, anchor string, enc position.Encoder) protocol.Location {
	docURI := uri.File(filepath.Join(idx.Root(), targetPath))
	rng := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 0, Character: 0},
	}
	if anchor != "" {
		doc := idx.GetByPath(targetPath)
		if doc != nil {
			if h := findHeading(doc, anchor); h != nil {
				rng = rangeToProtocol(idx, targetPath, h.Range, enc)
			}
		}
	}
	return protocol.Location{URI: docURI, Range: rng}
}

func targetLocationBlock(idx *index.Index, targetPath, blockID string, enc position.Encoder) protocol.Location {
	docURI := uri.File(filepath.Join(idx.Root(), targetPath))
	rng := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 0, Character: 0},
	}
	doc := idx.GetByPath(targetPath)
	if doc != nil {
		for _, b := range doc.Blocks {
			if b != nil && b.ID == blockID {
				rng = rangeToProtocol(idx, targetPath, b.Range, enc)
				break
			}
		}
	}
	return protocol.Location{URI: docURI, Range: rng}
}

func rangeToProtocol(idx *index.Index, relPath string, r parse.Range, enc position.Encoder) protocol.Range {
	content, err := idx.GetContent(relPath)
	if err != nil {
		return protocol.Range{}
	}
	lines := strings.Split(content, "\n")
	startChar := enc.ByteToChar(lineAt(lines, r.Start.Line), r.Start.Character)
	endChar := enc.ByteToChar(lineAt(lines, r.End.Line), r.End.Character)
	return protocol.Range{
		Start: protocol.Position{Line: uint32(r.Start.Line), Character: uint32(startChar)},
		End:   protocol.Position{Line: uint32(r.End.Line), Character: uint32(endChar)},
	}
}

func lineAt(lines []string, i int) string {
	if i < 0 || i >= len(lines) {
		return ""
	}
	return lines[i]
}
