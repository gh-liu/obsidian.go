package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/internal/lsp/position"
	"github.com/gh-liu/obsidian.go/parse"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// docSymbolNode holds a DocumentSymbol and its children for tree building.
type docSymbolNode struct {
	symbol   protocol.DocumentSymbol
	children []*docSymbolNode
}

// ResolveDocumentSymbol returns document symbols (TOC) for the given file.
// Builds a tree from headings; Range covers the full section for folding, SelectionRange for highlight.
func ResolveDocumentSymbol(ctx context.Context, idx *index.Index, root, encoding string, params *protocol.DocumentSymbolParams) ([]protocol.DocumentSymbol, error) {
	if idx == nil || params == nil {
		return nil, nil
	}
	fullPath := uri.URI(params.TextDocument.URI).Filename()
	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return nil, nil
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "..") {
		return nil, nil
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, nil
	}
	doc, err := parse.Parse(content, rel)
	if err != nil {
		return nil, nil
	}
	if len(doc.Headings) == 0 {
		return nil, nil
	}

	lines := strings.Split(string(content), "\n")
	enc := position.Encoder{Encoding: encoding}

	// Compute section end line for each heading: content until next heading of same-or-higher level.
	sectionEndLines := make([]int, len(doc.Headings))
	for i := range doc.Headings {
		endLine := len(lines) - 1
		for j := i + 1; j < len(doc.Headings); j++ {
			if doc.Headings[j].Level <= doc.Headings[i].Level {
				endLine = doc.Headings[j].Range.Start.Line - 1
				break
			}
		}
		if endLine < doc.Headings[i].Range.Start.Line {
			endLine = doc.Headings[i].Range.Start.Line
		}
		sectionEndLines[i] = endLine
	}

	// Build tree: each heading is child of the nearest preceding heading with smaller level.
	nodes := make([]*docSymbolNode, len(doc.Headings))
	for i := range doc.Headings {
		h := doc.Headings[i]
		selRange := rangeToProtocolFromLines(lines, h.Range, enc)
		endLine := sectionEndLines[i]
		endChar := len(lineAt(lines, endLine))
		sectionRange := parse.Range{
			Start: h.Range.Start,
			End:   parse.Pos{Line: endLine, Character: endChar},
		}
		fullRange := rangeToProtocolFromLines(lines, sectionRange, enc)

		nodes[i] = &docSymbolNode{
			symbol: protocol.DocumentSymbol{
				Name:           h.Text,
				Detail:         "",
				Kind:           protocol.SymbolKindModule,
				Range:          fullRange,
				SelectionRange: selRange,
			},
		}
	}

	// Attach children to parents.
	var roots []*docSymbolNode
	for i, n := range nodes {
		parent := -1
		for j := i - 1; j >= 0; j-- {
			if doc.Headings[j].Level < doc.Headings[i].Level {
				parent = j
				break
			}
		}
		if parent < 0 {
			roots = append(roots, n)
		} else {
			nodes[parent].children = append(nodes[parent].children, n)
		}
	}

	// Flatten to []DocumentSymbol with Children.
	out := make([]protocol.DocumentSymbol, 0, len(roots))
	for _, r := range roots {
		out = append(out, nodeToSymbol(r))
	}
	return out, nil
}

func nodeToSymbol(n *docSymbolNode) protocol.DocumentSymbol {
	s := n.symbol
	if len(n.children) > 0 {
		s.Children = make([]protocol.DocumentSymbol, 0, len(n.children))
		for _, c := range n.children {
			s.Children = append(s.Children, nodeToSymbol(c))
		}
	}
	return s
}

func rangeToProtocolFromLines(lines []string, r parse.Range, enc position.Encoder) protocol.Range {
	startChar := enc.ByteToChar(lineAt(lines, r.Start.Line), r.Start.Character)
	endChar := enc.ByteToChar(lineAt(lines, r.End.Line), r.End.Character)
	return protocol.Range{
		Start: protocol.Position{Line: uint32(r.Start.Line), Character: uint32(startChar)},
		End:   protocol.Position{Line: uint32(r.End.Line), Character: uint32(endChar)},
	}
}
