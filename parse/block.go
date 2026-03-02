package parse

import "regexp"

// Block represents a block reference (#^block-id).
type Block struct {
	ID    string
	Range Range
}

// Block ID: space + ^ + alphanumeric (Obsidian: end of block, e.g. "line content ^abc123")
var reBlockID = regexp.MustCompile(`\s+\^([a-zA-Z0-9_-]+)\s*$`)

func parseBlockID(line string, lineIdx int) *Block {
	m := reBlockID.FindStringSubmatchIndex(line)
	if m == nil {
		return nil
	}
	return &Block{
		ID: line[m[2]:m[3]],
		Range: Range{
			Start: Pos{Line: lineIdx, Character: m[0]},
			End:   Pos{Line: lineIdx, Character: len(line)},
		},
	}
}
