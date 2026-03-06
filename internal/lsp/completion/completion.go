package completion

import (
	"context"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/parse"
	"go.lsp.dev/protocol"
)

const maxCompletionItems = 100

// wikiLinkContext describes the completion context inside a wiki link [[...]].
type wikiLinkContext struct {
	startLine, startChar int
	prefix               string
	completeFiles        bool
	completeBlocks       bool
	completeAlias        bool
	targetPath           string
	targetAnchor         string
}

func toWikiLinkContext(lineIdx int, ctx *parse.WikiLinkCursorContext) *wikiLinkContext {
	if ctx == nil {
		return nil
	}
	return &wikiLinkContext{
		startLine:      lineIdx,
		startChar:      ctx.StartByte,
		prefix:         ctx.Prefix,
		completeFiles:  ctx.CompleteFiles,
		completeBlocks: ctx.CompleteBlock,
		completeAlias:  ctx.CompleteAlias,
		targetPath:     ctx.TargetPath,
		targetAnchor:   ctx.TargetAnchor,
	}
}

// ResolveCompletion returns completion items for Obsidian wiki links.
// Trigger: [[ (file completion), [[# (current-file heading), [[path# (heading of path).
func ResolveCompletion(ctx context.Context, idx *index.Index, relPath, encoding string, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	if idx == nil || params == nil {
		return nil, nil
	}

	reqCtx, ok := buildRequestContext(idx, relPath, encoding, params)
	if !ok {
		return nil, nil
	}
	byteOff := reqCtx.enc.CharToByte(reqCtx.line, reqCtx.cursorChar)

	linkCtx := toWikiLinkContext(reqCtx.lineIdx, parse.ParseWikiLinkCursorContext(reqCtx.line, byteOff))
	if linkCtx == nil {
		// Cursor after a single [ that may become [[.
		// Return IsIncomplete to keep the client session open so the next
		// keystroke triggers a re-query.
		if byteOff > 0 && reqCtx.line[byteOff-1] == '[' {
			return &protocol.CompletionList{IsIncomplete: true}, nil
		}
		return nil, nil
	}
	var items []protocol.CompletionItem
	if linkCtx.completeFiles {
		items = completeFiles(idx, linkCtx, reqCtx)
	} else if linkCtx.completeAlias {
		items = completeAliases(idx, linkCtx, reqCtx)
	} else if linkCtx.completeBlocks {
		items = completeBlocks(idx, reqCtx.currentRel, linkCtx, reqCtx)
	} else {
		items = completeHeadings(idx, reqCtx.currentRel, linkCtx, reqCtx)
	}
	isIncomplete := false
	if linkCtx.completeFiles && len(items) > maxCompletionItems {
		items = items[:maxCompletionItems]
		// Some clients suppress popup when IsIncomplete=true with empty prefix.
		// Keep empty-prefix completion visible, and use incomplete mode only when
		// user has started typing a filter.
		isIncomplete = linkCtx.prefix != ""
	}
	return &protocol.CompletionList{
		IsIncomplete: isIncomplete,
		Items:        items,
	}, nil
}
