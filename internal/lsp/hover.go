package lsp

import (
	"context"
	"fmt"
	"strings"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/internal/lsp/position"
	"github.com/gh-liu/obsidian.go/parse"
	"go.lsp.dev/protocol"
)

const hoverPreviewLines = 5

// ResolveHover resolves textDocument/hover for Obsidian wiki links.
func ResolveHover(ctx context.Context, idx *index.Index, relPath, encoding string, params *protocol.HoverParams) (*protocol.Hover, error) {
	if idx == nil || params == nil {
		return nil, nil
	}
	enc := position.Encoder{Encoding: encoding}
	rel, doc, lines := sourceContext(idx, relPath)
	if doc == nil {
		return nil, nil
	}
	lineIdx := int(params.Position.Line)
	if lineIdx < 0 || lineIdx >= len(lines) {
		return nil, nil
	}
	byteOff := enc.CharToByte(lines[lineIdx], int(params.Position.Character))

	link := linkAtPosition(doc, lineIdx, byteOff)
	if link == nil {
		return nil, nil
	}

	targetPath := rel
	if link.Target != "" {
		targetPath = idx.ResolveLinkTargetToPath(link.Target)
		if targetPath == "" {
			return nil, nil
		}
	}

	content, err := idx.GetContent(targetPath)
	if err != nil {
		return nil, nil
	}
	value := hoverMarkdown(idx.GetByPath(targetPath), targetPath, content, link.BlockRef)
	if value == "" {
		return nil, nil
	}
	rng := rangeToProtocol(idx, relPath, link.Range, enc)
	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: value,
		},
		Range: &rng,
	}, nil
}
func hoverMarkdown(doc *parse.Doc, targetPath, content, blockRef string) string {
	title := targetPath
	if doc != nil && doc.Title != "" {
		title = doc.Title
	}
	lines := previewLines(content, hoverPreviewLines)
	if blockRef != "" {
		lines = previewBlockLines(doc, blockRef)
	}
	if len(lines) == 0 {
		return fmt.Sprintf("**%s**\n\n`%s`", title, targetPath)
	}
	return fmt.Sprintf("**%s**\n\n`%s`\n\n---\n%s", title, targetPath, strings.Join(lines, "\n"))
}

func previewBlockLines(doc *parse.Doc, blockRef string) []string {
	if doc == nil {
		return nil
	}
	for _, b := range doc.Blocks {
		if b == nil || b.ID != blockRef {
			continue
		}
		preview := strings.TrimSpace(b.Preview)
		if preview == "" {
			return nil
		}
		lines := strings.Split(preview, "\n")
		if len(lines) > hoverPreviewLines {
			lines = lines[:hoverPreviewLines]
		}
		return lines
	}
	return nil
}

func previewLines(content string, limit int) []string {
	var out []string
	inFrontmatter := false
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if i == 0 && trimmed == "---" {
			inFrontmatter = true
			continue
		}
		if inFrontmatter {
			if trimmed == "---" {
				inFrontmatter = false
			}
			continue
		}
		if trimmed == "" {
			if len(out) == 0 {
				continue
			}
			out = append(out, "")
		} else {
			out = append(out, line)
		}
		if len(out) >= limit {
			break
		}
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return out
}
