package completion

import (
	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/internal/lsp/position"
	"go.lsp.dev/protocol"
)

type requestContext struct {
	currentRel string
	line       string
	lineIdx    int
	cursorChar int
	enc        position.Encoder
}

func buildRequestContext(idx *index.Index, relPath, encoding string, params *protocol.CompletionParams) (requestContext, bool) {
	lines, err := idx.GetLines(relPath)
	if err != nil {
		return requestContext{}, false
	}
	cursorChar := int(params.Position.Character)
	lineIdx := int(params.Position.Line)
	if lineIdx < 0 || lineIdx >= len(lines) {
		return requestContext{}, false
	}
	return requestContext{
		currentRel: relPath,
		line:       lines[lineIdx],
		lineIdx:    lineIdx,
		cursorChar: cursorChar,
		enc:        position.Encoder{Encoding: encoding},
	}, true
}

func buildReplaceRange(ctx *wikiLinkContext, reqCtx requestContext) protocol.Range {
	startChar := reqCtx.enc.ByteToChar(reqCtx.line, ctx.startChar)
	return protocol.Range{
		Start: protocol.Position{Line: uint32(ctx.startLine), Character: uint32(startChar)},
		End:   protocol.Position{Line: uint32(reqCtx.lineIdx), Character: uint32(reqCtx.cursorChar)},
	}
}
