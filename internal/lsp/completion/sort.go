package completion

import (
	"sort"

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
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		if scored[i].sortKey != scored[j].sortKey {
			return scored[i].sortKey < scored[j].sortKey
		}
		return scored[i].item.InsertText < scored[j].item.InsertText
	})
	items := make([]protocol.CompletionItem, 0, len(scored))
	for _, entry := range scored {
		items = append(items, entry.item)
	}
	return items
}
