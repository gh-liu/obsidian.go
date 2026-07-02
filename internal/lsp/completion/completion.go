package completion

import (
	"context"
	"strings"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/internal/lsp/position"
	"go.lsp.dev/protocol"
)

const maxItems = 100

// ResolveCompletion returns completion items for Obsidian wiki links.
func ResolveCompletion(ctx context.Context, idx *index.Index, relPath, encoding string, params *protocol.CompletionParams, imagePaths ...[]string) (*protocol.CompletionList, error) {
	if idx == nil || params == nil {
		return nil, nil
	}

	reqCtx, ok := buildRequestContext(idx, relPath, encoding, params)
	if !ok {
		return nil, nil
	}

	byteOff := reqCtx.enc.CharToByte(reqCtx.line, reqCtx.cursorChar)
	linkCtx := parseCursorContext(reqCtx.line, byteOff)
	if linkCtx == nil {
		// Cursor after a single '[' — signal incomplete to keep session open
		if byteOff > 0 && byteOff <= len(reqCtx.line) && reqCtx.line[byteOff-1] == '[' {
			return &protocol.CompletionList{IsIncomplete: true}, nil
		}
		return nil, nil
	}

	var items []protocol.CompletionItem
	switch {
	case linkCtx.completeImages:
		var paths []string
		if len(imagePaths) > 0 {
			paths = imagePaths[0]
		}
		items = completeImages(idx, linkCtx.prefix, paths)
	case linkCtx.completeFiles:
		items = completeFiles(idx, linkCtx.prefix)
	case linkCtx.completeAlias:
		items = completeAliases(idx, linkCtx.targetPath, linkCtx.prefix)
	case linkCtx.completeBlocks:
		items = completeBlocks(idx, linkCtx.targetPath, reqCtx.currentRel, linkCtx.prefix)
	default:
		items = completeHeadings(idx, linkCtx.targetPath, reqCtx.currentRel, linkCtx.prefix)
	}

	isIncomplete := false
	if linkCtx.completeFiles && len(items) > maxItems {
		items = items[:maxItems]
		isIncomplete = linkCtx.prefix != ""
	}

	return &protocol.CompletionList{
		IsIncomplete: isIncomplete,
		Items:        items,
	}, nil
}

// cursorContext mirrors parse.WikiLinkCursorContext for completion decisions.
type cursorContext struct {
	prefix         string
	completeFiles  bool
	completeImages bool
	completeBlocks bool
	completeAlias  bool
	targetPath     string // for heading/block/alias of a specific target
}

func parseCursorContext(line string, byteOff int) *cursorContext {
	ctx := parseWikiLinkCursorContext(line, byteOff)
	if ctx == nil {
		return nil
	}
	return &cursorContext{
		prefix:         ctx["prefix"],
		completeFiles:  ctx["completeFiles"] == "true",
		completeImages: ctx["completeImages"] == "true",
		completeBlocks: ctx["completeBlocks"] == "true",
		completeAlias:  ctx["completeAlias"] == "true",
		targetPath:     ctx["targetPath"],
	}
}

// parseWikiLinkCursorContext is a simplified in-package version of parse.ParseWikiLinkCursorContext
// to avoid depending on parse for completion context logic.
func parseWikiLinkCursorContext(line string, byteOff int) map[string]string {
	// Find nearest [[ that contains or precedes the cursor
	linkStart := -1
	linkEnd := -1

	for i := 0; i < len(line)-1; i++ {
		if line[i] != '[' || line[i+1] != '[' {
			continue
		}
		if i > 0 && line[i-1] == '\\' {
			continue
		}
		// Find closing ]]
		closeEnd := -1
		for j := i + 2; j < len(line)-1; j++ {
			if line[j] == '\\' && j+1 < len(line) {
				j++
				continue
			}
			if line[j] == ']' && line[j+1] == ']' {
				closeEnd = j + 2
				break
			}
		}
		if closeEnd > 0 && byteOff >= i && byteOff <= closeEnd {
			linkStart = i
			linkEnd = closeEnd - 2
			// Cursor past ]], no completion
			if byteOff >= linkEnd+2 {
				return nil
			}
			break
		}
		if closeEnd < 0 {
			if byteOff >= i+2 && byteOff <= len(line) {
				linkStart = i
				linkEnd = byteOff
				break
			}
		}
	}

	if linkStart < 0 {
		return nil
	}

	inner := ""
	if linkEnd > linkStart+2 {
		inner = line[linkStart+2 : linkEnd]
	}

	result := map[string]string{
		"prefix":         inner,
		"completeFiles":  "false",
		"completeImages": "false",
		"completeBlocks": "false",
		"completeAlias":  "false",
		"targetPath":     "",
	}

	target, anchorBlock, _ := parseWikiLinkParts(inner)
	afterHash := strings.LastIndex(inner, "#")
	afterPipe := strings.LastIndex(inner, "|")
	afterCaret := strings.LastIndex(inner, "^")

	if afterPipe >= 0 {
		aliasStart := afterPipe + 1
		if byteOff-linkStart-2 >= aliasStart {
			result["completeAlias"] = "true"
			result["targetPath"] = target
			result["prefix"] = strings.TrimPrefix(inner[aliasStart:], " ")
			return result
		}
	}

	if afterHash >= 0 {
		if afterCaret >= 0 && byteOff-linkStart-2 >= afterCaret {
			result["completeBlocks"] = "true"
			result["targetPath"] = target
			result["prefix"] = strings.TrimPrefix(inner[afterCaret+1:], " ")
			return result
		}
		result["targetPath"] = target
		result["prefix"] = inner[afterHash+1:]
		_ = anchorBlock
		return result
	}

	if linkStart > 0 && line[linkStart-1] == '!' {
		result["completeImages"] = "true"
	} else {
		result["completeFiles"] = "true"
	}
	result["prefix"] = target
	return result
}

func parseWikiLinkParts(inner string) (target, anchorBlock, alias string) {
	if idx := strings.LastIndex(inner, "|"); idx >= 0 {
		alias = strings.TrimSpace(inner[idx+1:])
		inner = inner[:idx]
	}
	if idx := strings.Index(inner, "#"); idx >= 0 {
		target = strings.TrimSpace(inner[:idx])
		anchorBlock = inner[idx:]
		return
	}
	target = strings.TrimSpace(inner)
	return
}

func buildRequestContext(idx *index.Index, relPath, encoding string, params *protocol.CompletionParams) (*struct {
	enc        position.Encoder
	line       string
	cursorChar int
	lineIdx    int
	currentRel string
}, bool) {
	doc := idx.GetByPath(relPath)
	if doc == nil {
		return nil, false
	}
	lines, err := idx.GetLines(relPath)
	if err != nil {
		return nil, false
	}
	lineIdx := int(params.Position.Line)
	if lineIdx < 0 || lineIdx >= len(lines) {
		return nil, false
	}
	return &struct {
		enc        position.Encoder
		line       string
		cursorChar int
		lineIdx    int
		currentRel string
	}{
		enc:        position.Encoder{Encoding: encoding},
		line:       lines[lineIdx],
		cursorChar: int(params.Position.Character),
		lineIdx:    lineIdx,
		currentRel: relPath,
	}, true
}
