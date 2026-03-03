package format

import "go.lsp.dev/protocol"

// DefaultOps defines the pipeline of format operations. Extend by appending more ops.
var DefaultOps = []FormatOp{
	FrontmatterOp,
}

// Run runs format ops in sequence and returns assembled TextEdits.
func Run(content string, ctx FormatContext, ops []FormatOp) []protocol.TextEdit {
	var edits []protocol.TextEdit
	for _, op := range ops {
		edit, newContent := op(content, ctx)
		content = newContent
		if edit != nil {
			edits = append(edits, *edit)
		}
	}
	return edits
}
