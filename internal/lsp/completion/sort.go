package completion

import (
	"cmp"
	"slices"

	"go.lsp.dev/protocol"
)

type scoredItem struct {
	item    protocol.CompletionItem
	score   int
	sortKey string
}

func sortItems(scored []scoredItem) []protocol.CompletionItem {
	if len(scored) == 0 {
		return nil
	}
	slices.SortFunc(scored, func(a, b scoredItem) int {
		if c := cmp.Compare(b.score, a.score); c != 0 {
			return c
		}
		if c := cmp.Compare(a.sortKey, b.sortKey); c != 0 {
			return c
		}
		return cmp.Compare(a.item.InsertText, b.item.InsertText)
	})
	items := make([]protocol.CompletionItem, 0, len(scored))
	for _, entry := range scored {
		items = append(items, entry.item)
	}
	return items
}
