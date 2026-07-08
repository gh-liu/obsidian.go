package lsp

import (
	"context"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/internal/lsp/position"
	"go.lsp.dev/protocol"
)

// ResolveDocumentSymbol returns the heading tree as LSP DocumentSymbols.
func ResolveDocumentSymbol(ctx context.Context, idx *index.Index, relPath, encoding string, params *protocol.DocumentSymbolParams) ([]protocol.DocumentSymbol, error) {
	enc := position.Encoder{Encoding: encoding}
	doc := idx.GetByPath(relPath)
	if doc == nil || len(doc.Headings) == 0 {
		return nil, nil
	}

	type entry struct {
		sym    protocol.DocumentSymbol
		parent int // index in entries, -1 for root
	}
	entries := make([]entry, 0, len(doc.Headings))
	lastAtLevel := make([]int, 7) // last entry index at each level, 0 = unset
	for i := range lastAtLevel {
		lastAtLevel[i] = -1
	}

	for _, h := range doc.Headings {
		if h == nil {
			continue
		}
		rng := rangeToProtocol(idx, relPath, h.Range, enc)
		e := entry{
			sym: protocol.DocumentSymbol{
				Name:           h.Text,
				Kind:           protocol.SymbolKindString,
				Range:          rng,
				SelectionRange: rng,
			},
			parent: -1,
		}
		// Find parent: highest level < current with a known entry
		for l := h.Level - 1; l >= 1; l-- {
			if lastAtLevel[l] >= 0 {
				e.parent = lastAtLevel[l]
				break
			}
		}
		entries = append(entries, e)
		idx := len(entries) - 1
		lastAtLevel[h.Level] = idx
		// Clear deeper levels
		for l := h.Level + 1; l <= 6; l++ {
			lastAtLevel[l] = -1
		}
	}

	// Build tree: record parent→child index mapping first, then resolve recursively.
	childIndices := make([][]int, len(entries))
	for i, e := range entries {
		if e.parent >= 0 {
			childIndices[e.parent] = append(childIndices[e.parent], i)
		}
	}

	symbols := make([]protocol.DocumentSymbol, len(entries))
	for i := range entries {
		symbols[i] = entries[i].sym
	}
	var resolve func(idx int)
	resolve = func(idx int) {
		for _, childIdx := range childIndices[idx] {
			resolve(childIdx)
			symbols[idx].Children = append(symbols[idx].Children, symbols[childIdx])
		}
	}

	// Return roots
	var roots []protocol.DocumentSymbol
	for i, e := range entries {
		if e.parent < 0 {
			resolve(i)
			roots = append(roots, symbols[i])
		}
	}
	return roots, nil
}
