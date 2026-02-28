package parse

import "time"

// Range is a 0-based [start, end) span in the document.
type Range struct {
	Start Pos
	End   Pos
}

// Pos is a 0-based position (line, character).
type Pos struct {
	Line      int
	Character int
}

// Doc is the parsed result of a single markdown file.
type Doc struct {
	Path string

	ID        string
	Tags      []string
	Aliases   []string
	CreatedAt time.Time
	UpdatedAt time.Time

	Headings []Heading
	Links    []Link
}
