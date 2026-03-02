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
// Returns target file location, or target file + heading if link has #anchor.
func ResolveDefinition(ctx context.Context, idx *index.Index, root, encoding string, params *protocol.DefinitionParams) ([]protocol.Location, error) {
	if idx == nil || params == nil {
		return nil, nil
	}
	enc := position.Encoder{Encoding: encoding}

	rel, doc, lines := sourceContext(idx, root, params)
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
	if link.Target != nil {
		targetPath = idx.ResolveLinkTargetToPath(link.Target.ID)
		if targetPath == "" {
			return nil, nil
		}
	} else {
		// Same-note link [[#heading]] or [[#^block-id]]: target is current file
		targetPath = rel
	}

	var loc protocol.Location
	if link.Block != nil {
		loc = targetLocationBlock(idx, root, targetPath, link.Block.ID, enc)
	} else if link.Anchor != nil {
		loc = targetLocation(idx, root, targetPath, link.Anchor.Text, enc)
	} else {
		loc = targetLocation(idx, root, targetPath, "", enc)
	}
	return []protocol.Location{loc}, nil
}

// sourceContext returns (relPath, doc, lines) for the file in params.
func sourceContext(idx *index.Index, root string, params *protocol.DefinitionParams) (string, *parse.Doc, []string) {
	fullPath := uri.URI(params.TextDocument.URI).Filename()
	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return "", nil, nil
	}
	rel = filepath.ToSlash(rel)
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

// linkAtPosition returns the link containing (lineIdx, byteOff), or nil.
// Includes both cross-note [[file#anchor]] and same-note [[#anchor]] links.
func linkAtPosition(doc *parse.Doc, lineIdx, byteOff int) *parse.Link {
	for _, link := range doc.Links {
		if link != nil && inRange(lineIdx, byteOff, link.Range) {
			return link
		}
	}
	return nil
}

func inRange(line, byteOff int, r parse.Range) bool {
	if line < r.Start.Line || (line == r.Start.Line && byteOff < r.Start.Character) {
		return false
	}
	if line > r.End.Line || (line == r.End.Line && byteOff >= r.End.Character) {
		return false
	}
	return true
}

// targetLocation builds protocol.Location for targetPath, optionally at heading anchor.
func targetLocation(idx *index.Index, root, targetPath, anchor string, enc position.Encoder) protocol.Location {
	uri := uri.File(filepath.Join(root, targetPath))
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
	return protocol.Location{URI: uri, Range: rng}
}

// targetLocationBlock builds protocol.Location for targetPath at block ID.
func targetLocationBlock(idx *index.Index, root, targetPath, blockID string, enc position.Encoder) protocol.Location {
	uri := uri.File(filepath.Join(root, targetPath))
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
	return protocol.Location{URI: uri, Range: rng}
}

func findHeading(doc *parse.Doc, anchor string) *parse.Heading {
	norm := normalizeHeadingAnchor(anchor)
	for _, h := range doc.Headings {
		if h != nil && (strings.EqualFold(anchor, h.Text) || normalizeHeadingAnchor(h.Text) == norm) {
			return h
		}
	}
	return nil
}

func normalizeHeadingAnchor(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "\t", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
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
