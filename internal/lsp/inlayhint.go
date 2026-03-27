package lsp

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/internal/lsp/position"
	"github.com/gh-liu/obsidian.go/parse"
	"go.lsp.dev/protocol"
)

const methodTextDocumentInlayHint = "textDocument/inlayHint"

type InlayHintParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	Range        protocol.Range                  `json:"range"`
}

type InlayHint struct {
	Position     protocol.Position `json:"position"`
	Label        any               `json:"label"`
	PaddingLeft  bool              `json:"paddingLeft,omitempty"`
	PaddingRight bool              `json:"paddingRight,omitempty"`
}

func (h *Handler) InlayHint(ctx context.Context, params *InlayHintParams) ([]InlayHint, error) {
	if h.index == nil || params == nil {
		return nil, nil
	}
	rel := uriToRelPath(params.TextDocument.URI, h.index.Root())
	if rel == "" {
		return nil, nil
	}
	return ResolveInlayHint(ctx, h.index, rel, h.positionEncoding, params)
}

func ResolveInlayHint(ctx context.Context, idx *index.Index, relPath, encoding string, params *InlayHintParams) ([]InlayHint, error) {
	if idx == nil || params == nil {
		return nil, nil
	}
	_ = ctx

	_, doc, lines := sourceContext(idx, relPath)
	if doc == nil {
		return nil, nil
	}
	enc := position.Encoder{Encoding: encoding}

	hints := make([]InlayHint, 0, len(doc.Links))
	for _, link := range doc.Links {
		if link == nil || link.Kind != parse.LinkWiki {
			continue
		}
		if !linkIntersectsProtocolRange(link, params.Range, lines, enc) {
			continue
		}
		label := wikiLinkInlayLabel(idx, relPath, link)
		if label == "" {
			continue
		}
		line := lineAt(lines, link.Range.End.Line)
		hintByte := link.Range.End.Character
		if hintByte >= 2 {
			hintByte -= 2
		}
		hints = append(hints, InlayHint{
			Position: protocol.Position{
				Line:      uint32(link.Range.End.Line),
				Character: uint32(enc.ByteToChar(line, hintByte)),
			},
			Label:       formatWikiLinkInlayHintLabel(label, link.Alias),
			PaddingLeft: true,
		})
	}
	if len(hints) == 0 {
		return nil, nil
	}
	return hints, nil
}

func wikiLinkInlayLabel(idx *index.Index, sourcePath string, link *parse.Link) string {
	if idx == nil || link == nil || link.Kind != parse.LinkWiki {
		return ""
	}
	targetPath := sourcePath
	if link.Target != "" {
		targetPath = idx.ResolveLinkTargetToPath(link.Target)
		if targetPath == "" {
			return ""
		}
	}
	targetDoc := idx.GetByPath(targetPath)

	var b strings.Builder
	if base := inlayHintTargetLabel(targetDoc, targetPath, link.Target != ""); base != "" {
		b.WriteString(base)
	}

	if link.BlockRef != "" {
		if b.Len() > 0 {
			b.WriteString("#^")
		} else {
			b.WriteByte('#')
			b.WriteByte('^')
		}
		b.WriteString(link.BlockRef)
		return b.String()
	}

	if link.Anchor == "" {
		return b.String()
	}

	anchor := headingLabelForHint(targetDoc, link.Anchor)
	if anchor == "" {
		anchor = link.Anchor
	}
	b.WriteByte('#')
	b.WriteString(anchor)
	return b.String()
}

func formatWikiLinkInlayHintLabel(label, alias string) string {
	if normalizedInlayHintText(alias) != "" && normalizedInlayHintText(alias) == normalizedInlayHintText(label) {
		return "->"
	}
	return "-> " + label
}

func normalizedInlayHintText(s string) string {
	s = strings.ReplaceAll(s, "->", "")
	return strings.TrimSpace(s)
}

func inlayHintTargetLabel(doc *parse.Doc, targetPath string, includePathFallback bool) string {
	if doc != nil && strings.TrimSpace(doc.Title) != "" {
		return strings.TrimSpace(doc.Title)
	}
	if includePathFallback {
		return trimMarkdownExt(targetPath)
	}
	return ""
}

func headingLabelForHint(doc *parse.Doc, anchor string) string {
	if doc == nil || anchor == "" {
		return ""
	}
	heading := findHeading(doc, anchor)
	if heading == nil {
		return ""
	}
	return strings.TrimSpace(heading.Text)
}

func trimMarkdownExt(path string) string {
	if strings.HasSuffix(strings.ToLower(path), ".md") {
		return path[:len(path)-3]
	}
	return path
}

func linkIntersectsProtocolRange(link *parse.Link, want protocol.Range, lines []string, enc position.Encoder) bool {
	if link == nil {
		return false
	}
	got := protocol.Range{
		Start: protocol.Position{
			Line:      uint32(link.Range.Start.Line),
			Character: uint32(enc.ByteToChar(lineAt(lines, link.Range.Start.Line), link.Range.Start.Character)),
		},
		End: protocol.Position{
			Line:      uint32(link.Range.End.Line),
			Character: uint32(enc.ByteToChar(lineAt(lines, link.Range.End.Line), link.Range.End.Character)),
		},
	}
	return protocolRangeIntersects(got, want)
}

func protocolRangeIntersects(a, b protocol.Range) bool {
	return protocolPositionLess(a.Start, b.End) && protocolPositionLess(b.Start, a.End)
}

func protocolPositionLess(a, b protocol.Position) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Character < b.Character
}

func marshalInitializeResultWithInlayHint(result *protocol.InitializeResult) (map[string]any, error) {
	if result == nil {
		return map[string]any{
			"capabilities": map[string]any{
				"inlayHintProvider": true,
			},
		}, nil
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	caps, _ := raw["capabilities"].(map[string]any)
	if caps == nil {
		caps = make(map[string]any)
		raw["capabilities"] = caps
	}
	caps["inlayHintProvider"] = true
	return raw, nil
}
