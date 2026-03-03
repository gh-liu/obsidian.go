package format

import (
	"strings"

	"github.com/gh-liu/obsidian.go/internal/lsp/position"
	"github.com/gh-liu/obsidian.go/internal/lsp/template"
	"go.lsp.dev/protocol"
)

// FormatContext provides context for format operations.
type FormatContext struct {
	Path  string
	Title string
	Enc   position.Encoder
}

// FormatOp is a format operation. Returns (edit, newContent). Edit is nil when no change.
type FormatOp func(content string, ctx FormatContext) (*protocol.TextEdit, string)

// FrontmatterOp ensures frontmatter has default fields (id, title, createdAt, updatedAt).
// Edit range is limited to frontmatter only.
func FrontmatterOp(content string, ctx FormatContext) (*protocol.TextEdit, string) {
	formatted := template.EnsureFrontmatterDefaults(content, ctx.Title)
	if formatted == content {
		return nil, content
	}
	start, end := frontmatterRange(content, ctx.Enc)
	newFm := formattedFrontmatter(formatted)
	if start.Line == 0 && start.Character == 0 && end.Line == 0 && end.Character == 0 {
		newFm += "\n"
	}
	return &protocol.TextEdit{
		Range: protocol.Range{
			Start: protocol.Position{Line: uint32(start.Line), Character: uint32(start.Character)},
			End:   protocol.Position{Line: uint32(end.Line), Character: uint32(end.Character)},
		},
		NewText: newFm,
	}, formatted
}

type lineChar struct {
	Line      int
	Character int
}

func frontmatterRange(content string, enc position.Encoder) (start, end lineChar) {
	if !strings.HasPrefix(content, "---\n") {
		return lineChar{0, 0}, lineChar{0, 0}
	}
	idx := strings.Index(content[4:], "\n---")
	if idx < 0 {
		return lineChar{0, 0}, lineChar{0, 0}
	}
	fm := content[:4+idx+4]
	lines := strings.Split(fm, "\n")
	lastIdx := len(lines) - 1
	if lastIdx < 0 {
		return lineChar{0, 0}, lineChar{0, 0}
	}
	lastChar := enc.ByteToChar(lines[lastIdx], len(lines[lastIdx]))
	return lineChar{0, 0}, lineChar{lastIdx, lastChar}
}

func formattedFrontmatter(formatted string) string {
	if !strings.HasPrefix(formatted, "---\n") {
		return ""
	}
	idx := strings.Index(formatted[4:], "\n---")
	if idx < 0 {
		return ""
	}
	return formatted[:4+idx+4]
}
