package completion

import (
	"path"
	"strings"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/protocol"
)

func completeAliases(idx *index.Index, wikiCtx *wikiLinkContext, reqCtx requestContext) []protocol.CompletionItem {
	candidates := resolveAliasCandidates(idx, reqCtx.currentRel, wikiCtx)
	if len(candidates) == 0 {
		return nil
	}
	prefixLower := strings.ToLower(wikiCtx.prefix)
	rng := buildReplaceRange(wikiCtx, reqCtx)
	scored := make([]scoredItem, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		score := headingMatchScore(prefixLower, strings.ToLower(candidate))
		if score == 0 {
			continue
		}
		item := protocol.CompletionItem{
			Label:      candidate,
			InsertText: candidate,
			FilterText: candidate,
			Kind:       protocol.CompletionItemKindText,
			TextEdit:   &protocol.TextEdit{Range: rng, NewText: candidate},
		}
		scored = append(scored, scoredItem{
			item:    item,
			score:   score,
			sortKey: strings.ToLower(candidate),
		})
	}
	return sortItems(scored)
}

func resolveAliasCandidates(idx *index.Index, currentRel string, wikiCtx *wikiLinkContext) []string {
	if wikiCtx.targetAnchor != "" {
		return []string{wikiCtx.targetAnchor}
	}

	docPath := currentRel
	if wikiCtx.targetPath != "" {
		docPath = idx.ResolveLinkTargetToPath(wikiCtx.targetPath)
		if docPath == "" {
			return nil
		}
	}
	doc := idx.GetByPath(docPath)
	if doc == nil {
		return nil
	}
	candidates := make([]string, 0, 2+len(doc.Aliases))
	if doc.Title != "" {
		candidates = append(candidates, doc.Title)
	}
	candidates = append(candidates, doc.Aliases...)
	display := strings.TrimSuffix(doc.Path, ".md")
	if display != "" {
		candidates = append(candidates, path.Base(display))
	}
	return candidates
}
