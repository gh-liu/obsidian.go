package parse

import "strings"

// parseHeading parses a markdown heading from a line. Returns nil if not a heading.
// Supports # through ######. The heading text is trimmed.
func parseHeading(line string, lineIdx int) *Heading {
	if !strings.HasPrefix(line, "#") {
		return nil
	}
	level := 0
	for _, r := range line {
		if r != '#' {
			break
		}
		level++
	}
	if level == 0 || level > 6 {
		return nil
	}
	if level >= len(line) || line[level] != ' ' {
		return nil
	}
	text := strings.TrimSpace(line[level:])
	return &Heading{
		Level: level,
		Text:  text,
		Range: Range{
			Start: Pos{Line: lineIdx, Character: 0},
			End:   Pos{Line: lineIdx, Character: len(line)},
		},
	}
}
