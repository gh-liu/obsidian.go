package completion

import (
	"strings"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/parse"
	"go.lsp.dev/protocol"
)

func completeBlocks(idx *index.Index, currentRel string, wikiCtx *wikiLinkContext, reqCtx requestContext) []protocol.CompletionItem {
	blocks := resolveBlockCandidates(idx, currentRel, wikiCtx.targetPath)
	if len(blocks) == 0 {
		return nil
	}
	prefixLower := strings.ToLower(wikiCtx.prefix)
	scored := make([]scoredItem, 0, len(blocks))
	if wikiCtx.startChar > len(reqCtx.line) {
		return nil
	}
	rng := buildReplaceRange(wikiCtx, reqCtx)

	for _, b := range blocks {
		if b == nil {
			continue
		}
		score := blockMatchScore(prefixLower, b.ID)
		if score == 0 {
			continue
		}
		item := protocol.CompletionItem{
			Label:      b.ID,
			InsertText: b.ID,
			FilterText: b.ID,
			Kind:       protocol.CompletionItemKindReference,
			TextEdit:   &protocol.TextEdit{Range: rng, NewText: b.ID},
		}
		scored = append(scored, scoredItem{
			item:    item,
			score:   score,
			sortKey: strings.ToLower(b.ID),
		})
	}
	return sortItems(scored)
}

func resolveBlockCandidates(idx *index.Index, currentRel, targetPath string) []*parse.Block {
	docPath := currentRel
	if targetPath != "" {
		docPath = idx.ResolveLinkTargetToPath(targetPath)
		if docPath == "" {
			return nil
		}
	}
	doc := idx.GetByPath(docPath)
	if doc == nil {
		return nil
	}
	return doc.Blocks
}
