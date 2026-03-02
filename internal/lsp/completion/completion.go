package completion

import (
	"context"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/parse"
	"go.lsp.dev/protocol"
)

// wikiLinkContext describes the completion context inside a wiki link [[...]].
type wikiLinkContext struct {
	startLine, startChar int
	prefix               string
	completeFiles        bool
	targetPath           string
}

func toWikiLinkContext(lineIdx int, ctx *parse.WikiLinkCursorContext) *wikiLinkContext {
	if ctx == nil {
		return nil
	}
	return &wikiLinkContext{
		startLine:     lineIdx,
		startChar:     ctx.StartByte,
		prefix:        ctx.Prefix,
		completeFiles: ctx.CompleteFiles,
		targetPath:    ctx.TargetPath,
	}
}

// ResolveCompletion returns completion items for Obsidian wiki links.
// Trigger: [[ (file completion), [[# (current-file heading), [[path# (heading of path).
func ResolveCompletion(ctx context.Context, idx *index.Index, root, encoding string, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	if idx == nil || params == nil {
		return nil, nil
	}

	reqCtx, ok := buildRequestContext(idx, root, encoding, params)
	if !ok {
		return nil, nil
	}
	byteOff := reqCtx.enc.CharToByte(reqCtx.line, reqCtx.cursorChar)

	linkCtx := toWikiLinkContext(reqCtx.lineIdx, parse.ParseWikiLinkCursorContext(reqCtx.line, byteOff))
	if linkCtx == nil {
		return nil, nil
	}
	var items []protocol.CompletionItem
	if linkCtx.completeFiles {
		items = completeFiles(idx, linkCtx, reqCtx)
	} else {
		items = completeHeadings(idx, reqCtx.currentRel, linkCtx, reqCtx)
	}
	return &protocol.CompletionList{Items: items}, nil
}
