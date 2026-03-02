package parse

import (
	"regexp"
	"strings"
)

// Heading represents a markdown heading (h1-h6).
type Heading struct {
	Level int
	Text  string
	Range Range
}

var reHeading = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

func parseHeading(line string, lineIdx int) *Heading {
	m := reHeading.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	return &Heading{
		Level: len(m[1]),
		Text:  strings.TrimSpace(m[2]),
		Range: Range{
			Start: Pos{Line: lineIdx, Character: 0},
			End:   Pos{Line: lineIdx, Character: len(line)},
		},
	}
}
