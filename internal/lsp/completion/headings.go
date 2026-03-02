package completion

import (
	"strings"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/parse"
	"go.lsp.dev/protocol"
)

func completeHeadings(idx *index.Index, currentRel string, wikiCtx *wikiLinkContext, reqCtx requestContext) []protocol.CompletionItem {
	headings := resolveHeadingCandidates(idx, currentRel, wikiCtx.targetPath)
	if len(headings) == 0 {
		return nil
	}
	prefixLower := strings.ToLower(wikiCtx.prefix)
	scored := make([]scoredItem, 0, len(headings))
	if wikiCtx.startChar > len(reqCtx.line) {
		return nil
	}
	rng := buildReplaceRange(wikiCtx, reqCtx)

	for _, h := range headings {
		if h == nil {
			continue
		}
		score := headingMatchScore(prefixLower, h.Text)
		if score == 0 {
			continue
		}
		item := protocol.CompletionItem{
			Label:      h.Text,
			InsertText: h.Text,
			FilterText: h.Text,
			Kind:       protocol.CompletionItemKindReference,
			TextEdit:   &protocol.TextEdit{Range: rng, NewText: h.Text},
		}
		scored = append(scored, scoredItem{
			item:    item,
			score:   score,
			sortKey: strings.ToLower(h.Text),
		})
	}
	return sortItems(scored)
}

func resolveHeadingCandidates(idx *index.Index, currentRel, targetPath string) []*parse.Heading {
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
	return doc.Headings
}
