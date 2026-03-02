package completion

import (
	"strings"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/parse"
	"go.lsp.dev/protocol"
)

type fileCompletionItem struct {
	display    string
	insertText string
	filterText string
	detail     string
	kind       protocol.CompletionItemKind
}

func completeFiles(idx *index.Index, ctx *wikiLinkContext, reqCtx requestContext) []protocol.CompletionItem {
	prefixLower := strings.ToLower(ctx.prefix)
	var scored []scoredItem
	rng := buildReplaceRange(ctx, reqCtx)

	idx.RangePaths(func(p string, doc *parse.Doc) bool {
		if doc == nil {
			return true
		}
		display := strings.TrimSuffix(p, ".md")
		if display == "" {
			display = p
		}
		score := fileMatchScore(prefixLower, display, p, doc.Aliases)
		if score == 0 {
			return true
		}
		fc := buildFileCompletionItem(p, display, doc)
		item := protocol.CompletionItem{
			Label:      fc.display,
			InsertText: fc.insertText,
			FilterText: fc.filterText,
			Detail:     fc.detail,
			Kind:       fc.kind,
			TextEdit:   &protocol.TextEdit{Range: rng, NewText: fc.insertText},
		}
		scored = append(scored, scoredItem{
			item:    item,
			score:   score,
			sortKey: strings.ToLower(fc.display),
		})
		return true
	})
	return sortItems(scored)
}

func buildFileCompletionItem(path, display string, doc *parse.Doc) fileCompletionItem {
	insertText := display
	detail := ""
	if doc.ID != "" {
		insertText = doc.ID
		detail = "id: " + doc.ID
	}
	filterParts := []string{display, path}
	filterParts = append(filterParts, doc.Aliases...)
	return fileCompletionItem{
		display:    display,
		insertText: insertText,
		filterText: strings.Join(filterParts, " "),
		detail:     detail,
		kind:       protocol.CompletionItemKindFile,
	}
}
