package lsp

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/parse"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// wikiLinkContext describes the completion context inside a wiki link [[...]].
type wikiLinkContext struct {
	// startChar is the character offset of the first char after [[ (or [[#) to be replaced.
	startLine, startChar int
	// prefix is the text from start to cursor, used for filtering.
	prefix string
	// completeFiles: true = complete file paths; false = complete headings.
	completeFiles bool
	// targetPath: when completeFiles=false, the file whose headings we complete (empty = current file).
	targetPath string
}

// ResolveCompletion returns completion items for Obsidian wiki links.
// Trigger: [[ (file completion), [[# (current-file heading), [[path# (heading of path).
func ResolveCompletion(ctx context.Context, idx *index.Index, root, encoding string, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	if idx == nil || params == nil {
		return nil, nil
	}
	enc := PositionEncoder{encoding: encoding}

	docURI := params.TextDocument.URI
	fullPath := uri.URI(docURI).Filename()
	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return nil, nil
	}
	rel = filepath.ToSlash(rel)

	lines := docLines(idx, rel)
	lineIdx := int(params.Position.Line)
	if lineIdx < 0 || lineIdx >= len(lines) {
		return nil, nil
	}
	line := lines[lineIdx]
	cursorChar := int(params.Position.Character)
	byteOff := enc.CharToByte(line, cursorChar)

	// When triggered by "#", doc may not have "#" yet (DidChange race). Treat [[path as [[path#.
	parseLines := lines
	if params.Context != nil && params.Context.TriggerCharacter == "#" && byteOff <= len(line) && byteOff > 0 {
		modified := make([]string, len(lines))
		copy(modified, lines)
		modified[lineIdx] = line[:byteOff] + "#" + line[byteOff:]
		parseLines = modified
		line = modified[lineIdx]
		byteOff++
		cursorChar++
	}

	ctx2 := parseWikiLinkContext(parseLines, lineIdx, byteOff)
	if ctx2 == nil {
		return nil, nil
	}
	var items []protocol.CompletionItem
	if ctx2.completeFiles {
		items = completeFiles(idx, ctx2, enc, line, lineIdx, cursorChar)
	} else {
		items = completeHeadings(idx, root, rel, ctx2, enc, line, lineIdx, cursorChar)
	}
	if len(items) == 0 {
		return &protocol.CompletionList{Items: []protocol.CompletionItem{}}, nil
	}
	return &protocol.CompletionList{
		Items:        items,
		IsIncomplete: false,
	}, nil
}

// parseWikiLinkContext scans backwards from (lineIdx, byteOff) to find an unclosed [[.
// Returns nil if not inside a wiki link.
func parseWikiLinkContext(lines []string, lineIdx, byteOff int) *wikiLinkContext {
	// Collect content from line start (or last ]] on same line) up to cursor.
	// For simplicity: scan current line backwards for [[.
	line := lines[lineIdx]
	if byteOff > len(line) {
		byteOff = len(line)
	}
	beforeCursor := line[:byteOff]

	// Find last [[ that is not closed by ]] before cursor.
	open := -1
	for i := 0; i < len(beforeCursor)-1; i++ {
		if beforeCursor[i] == '[' && beforeCursor[i+1] == '[' {
			// Check if closed
			rest := beforeCursor[i+2 : byteOff]
			if !strings.Contains(rest, "]]") {
				open = i + 2 // position after [[
			}
		}
	}
	if open < 0 {
		return nil
	}

	inner := beforeCursor[open:byteOff]
	// Check for [[# (same-note heading)
	if strings.HasPrefix(inner, "#") {
		prefix := strings.TrimPrefix(inner, "#")
		// Handle nested #: [[#h1#h2]] - we complete the part after last #
		lastHash := strings.LastIndex(prefix, "#")
		if lastHash >= 0 {
			prefix = prefix[lastHash+1:]
			return &wikiLinkContext{
				startLine:     lineIdx,
				startChar:     open + 1 + lastHash + 1, // after last #
				prefix:        prefix,
				completeFiles: false,
				targetPath:    "",
			}
		}
		return &wikiLinkContext{
			startLine:     lineIdx,
			startChar:     open + 1, // after #
			prefix:        prefix,
			completeFiles: false,
			targetPath:    "",
		}
	}
	// Check for path# (heading of another file)
	if idx := strings.LastIndex(inner, "#"); idx >= 0 {
		targetPath := strings.TrimSpace(inner[:idx])
		prefix := inner[idx+1:]
		return &wikiLinkContext{
			startLine:     lineIdx,
			startChar:     open + idx + 1,
			prefix:        prefix,
			completeFiles: false,
			targetPath:    targetPath,
		}
	}
	// File path completion
	prefix := inner
	return &wikiLinkContext{
		startLine:     lineIdx,
		startChar:     open,
		prefix:        prefix,
		completeFiles: true,
	}
}

// docLines returns document lines from index (open-file cache or disk).
func docLines(idx *index.Index, rel string) []string {
	content, err := idx.GetContent(rel)
	if err != nil {
		return nil
	}
	return strings.Split(content, "\n")
}

func completeFiles(idx *index.Index, ctx *wikiLinkContext, enc PositionEncoder, line string, lineIdx, cursorChar int) []protocol.CompletionItem {
	paths := idx.ListPaths()
	prefix := ctx.prefix
	prefixLower := strings.ToLower(prefix)

	var items []protocol.CompletionItem
	for _, p := range paths {
		doc := idx.GetByPath(p)
		if doc == nil {
			continue
		}
		// Label/Filter: path (filename) for display and filtering - user types to find by name
		display := strings.TrimSuffix(p, ".md")
		if display == "" {
			display = p
		}
		if prefix != "" {
			dl, pl := strings.ToLower(display), strings.ToLower(p)
			if !strings.HasPrefix(dl, prefixLower) && !strings.HasPrefix(pl, prefixLower) && !strings.Contains(dl, prefixLower) {
				continue
			}
		}
		filterText := display
		// Insert: id when file has frontmatter id (Obsidian jump uses id); else path without .md
		insertText := display
		detail := ""
		if doc.ID != "" {
			insertText = doc.ID
			detail = "id: " + doc.ID
		}
		startChar := enc.ByteToChar(line, ctx.startChar)
		items = append(items, protocol.CompletionItem{
			Label:      display,
			InsertText: insertText,
			FilterText: filterText,
			Detail:     detail,
			Kind:       protocol.CompletionItemKindFile,
			TextEdit: &protocol.TextEdit{
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(ctx.startLine), Character: uint32(startChar)},
					End:   protocol.Position{Line: uint32(lineIdx), Character: uint32(cursorChar)},
				},
				NewText: insertText,
			},
		})
	}
	return items
}

func completeHeadings(idx *index.Index, root string, currentRel string, wikiCtx *wikiLinkContext, enc PositionEncoder, line string, lineIdx, cursorChar int) []protocol.CompletionItem {
	var headings []*parse.Heading
	if wikiCtx.targetPath == "" {
		// Current file: use index (has unsaved content from DidOpen/DidChange)
		doc := idx.GetByPath(currentRel)
		if doc != nil {
			headings = doc.Headings
		}
	} else {
		// Other file: use index
		resolved := idx.ResolveLinkTargetToPath(wikiCtx.targetPath)
		if resolved == "" {
			return nil
		}
		doc := idx.GetByPath(resolved)
		if doc != nil {
			headings = doc.Headings
		}
	}

	prefixLower := strings.ToLower(wikiCtx.prefix)
	var items []protocol.CompletionItem
	if wikiCtx.startChar > len(line) {
		return nil
	}
	startChar := enc.ByteToChar(line, wikiCtx.startChar)
	for _, h := range headings {
		if wikiCtx.prefix != "" && !strings.HasPrefix(strings.ToLower(h.Text), prefixLower) && !strings.Contains(strings.ToLower(h.Text), prefixLower) {
			continue
		}
		items = append(items, protocol.CompletionItem{
			Label:      h.Text,
			InsertText: h.Text,
			FilterText: h.Text,
			Kind:       protocol.CompletionItemKindReference,
			TextEdit: &protocol.TextEdit{
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(wikiCtx.startLine), Character: uint32(startChar)},
					End:   protocol.Position{Line: uint32(lineIdx), Character: uint32(cursorChar)},
				},
				NewText: h.Text,
			},
		})
	}
	return items
}
