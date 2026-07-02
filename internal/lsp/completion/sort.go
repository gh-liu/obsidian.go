package completion

import (
	"sort"
	"strings"

	"go.lsp.dev/protocol"
)

func stringContainsLower(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), substr)
}

func sortByRelevance(items []protocol.CompletionItem, filter string) {
	sort.SliceStable(items, func(i, j int) bool {
		a := strings.ToLower(strings.TrimSuffix(items[i].Label, ".md"))
		b := strings.ToLower(strings.TrimSuffix(items[j].Label, ".md"))
		if filter != "" {
			ai := strings.Index(a, filter)
			bi := strings.Index(b, filter)
			if ai >= 0 && bi >= 0 && ai != bi {
				return ai < bi
			}
			if ai >= 0 != (bi >= 0) {
				return ai >= 0
			}
		}
		// Shorter paths first, then alphabetical
		if len(a) != len(b) {
			return len(a) < len(b)
		}
		return a < b
	})
}
