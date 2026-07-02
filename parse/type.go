package parse

import (
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Range is a 0-based [start, end) span in the document.
type Range struct {
	Start Pos
	End   Pos
}

// Pos is a 0-based position (line, character). Character is UTF-8 byte offset.
type Pos struct {
	Line      int
	Character int
}

// Doc is the parsed result of a single markdown file.
type Doc struct {
	Path string

	ID        string
	Title     string
	Tags      []string
	Aliases   []string
	CreatedAt time.Time
	UpdatedAt time.Time

	Headings []*Heading
	Blocks   []*Block
	Links    []*Link
}

// Heading represents a markdown heading (# Title). Level is 1-based.
type Heading struct {
	Level int
	Text  string
	Range Range
}

// Block represents an explicit block ID marker (^block-id).
type Block struct {
	ID    string
	Range Range
}

// LinkKind distinguishes wiki links from markdown links.
type LinkKind int

const (
	LinkWiki LinkKind = iota
	LinkMarkdown
)

// Link represents a link to another note or resource.
// Target stores the link target (id or path). Empty Target means same-note link [[#heading]].
// Anchor stores heading text without '#'. BlockRef stores block id without '^'.
type Link struct {
	Kind     LinkKind
	Target   string
	Anchor   string
	BlockRef string
	Alias    string
	Range    Range
}

// --- frontmatter YAML helpers ---

var timeLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02",
	time.RFC3339,
}

type yamlTime struct {
	t time.Time
}

func (y *yamlTime) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return nil
	}
	s := strings.TrimSpace(n.Value)
	if s == "" {
		return nil
	}
	for _, layout := range timeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			y.t = t
			return nil
		}
	}
	return nil
}

type yamlStrings struct {
	values []string
}

func (y *yamlStrings) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		y.values = []string{n.Value}
		return nil
	}
	if n.Kind == yaml.SequenceNode {
		for _, c := range n.Content {
			if c.Kind == yaml.ScalarNode {
				y.values = append(y.values, c.Value)
			}
		}
		return nil
	}
	return nil
}

func (y *yamlStrings) Values() []string {
	if y == nil {
		return nil
	}
	return y.values
}
