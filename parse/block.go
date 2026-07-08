package parse

import "strings"

// parseBlockID parses an explicit block ID marker (^block-id) from a line.
// Returns nil if the line contains no block ID.
// Skips ^ inside wiki links (preceded by # inside [[...]]).
func parseBlockID(lines []string, idxInBody, lineIdx int) *Block {
	line := lines[idxInBody]
	idx := strings.LastIndex(line, "^")
	if idx < 0 {
		return nil
	}
	// Skip ^ inside wiki link block ref ([[file#^block-id]])
	if idx > 0 && line[idx-1] == '#' {
		return nil
	}
	// ^ at the end with nothing after it
	if idx == len(line)-1 {
		return nil
	}
	id := strings.TrimSpace(line[idx+1:])
	if id == "" {
		return nil
	}
	// If there are spaces, the ^ was not for a block ID
	if strings.Contains(id, " ") {
		return nil
	}
	preview := strings.TrimSpace(line[:idx])
	if preview == "" {
		if idxInBody == 0 || idxInBody == len(lines)-1 || strings.TrimSpace(lines[idxInBody-1]) != "" || strings.TrimSpace(lines[idxInBody+1]) != "" {
			return nil
		}
		for i := idxInBody - 2; i >= 0; i-- {
			if strings.TrimSpace(lines[i]) == "" {
				preview = strings.TrimSpace(strings.Join(lines[i+1:idxInBody-1], "\n"))
				break
			}
			if i == 0 {
				preview = strings.TrimSpace(strings.Join(lines[:idxInBody-1], "\n"))
			}
		}
		if preview == "" {
			return nil
		}
	}
	return &Block{
		ID:      id,
		Preview: preview,
		Range: Range{
			Start: Pos{Line: lineIdx, Character: idx},
			End:   Pos{Line: lineIdx, Character: len(line)},
		},
	}
}
