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
		ai := strings.ToLower(items[i].Label)
		bi := strings.ToLower(items[j].Label)
		if filter != "" {
			f := strings.ToLower(filter)
			// Prefer prefix matches over mid-string matches
			aiPrefix := strings.HasPrefix(ai, f)
			biPrefix := strings.HasPrefix(bi, f)
			if aiPrefix != biPrefix {
				return aiPrefix
			}
			// Among prefix matches, prefer shorter labels
			if aiPrefix {
				if len(ai) != len(bi) {
					return len(ai) < len(bi)
				}
				return ai < bi
			}
			// Mid-string: prefer earlier position
			aiPos := strings.Index(ai, f)
			biPos := strings.Index(bi, f)
			if aiPos >= 0 && biPos >= 0 && aiPos != biPos {
				return aiPos < biPos
			}
			if aiPos >= 0 != (biPos >= 0) {
				return aiPos >= 0
			}
		}
		if len(ai) != len(bi) {
			return len(ai) < len(bi)
		}
		return ai < bi
	})
}
