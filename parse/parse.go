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

	Headings []*Heading
	Blocks   []*Block // explicit ^block-id in document
	Links    []*Link
}

// Parse parses a markdown file into Doc. path is the relative path (used for mapping key).
// Link/Heading Range uses full-document line (0-based) and UTF-8 byte offset for Character.
// LSP clients may use utf-8 or utf-16; conversion happens at use site (e.g. Definition handler).
func Parse(content []byte, path string) (*Doc, error) {
	doc := &Doc{Path: path}

	fm, body := splitFrontmatter(content)
	bodyStartLine := 0
	if fm != nil {
		parseFrontmatter(fm, doc)
		bodyStartLine = strings.Count(string(content[:len(content)-len(body)]), "\n")
	}
	parseBody(body, bodyStartLine, doc)

	return doc, nil
}

func splitFrontmatter(content []byte) (frontmatter, body []byte) {
	s := string(content)
	if !strings.HasPrefix(s, "---\n") {
		return nil, content
	}
	before, after, ok := strings.Cut(s[4:], "\n---")
	if !ok {
		return nil, content
	}
	return []byte(before), []byte(after)
}

func parseFrontmatter(raw []byte, doc *Doc) {
	var flexible struct {
		ID        string      `yaml:"id"`
		Aliases   yamlAliases `yaml:"aliases"`
		Tags      yamlTags    `yaml:"tags"`
		CreatedAt yamlTime    `yaml:"createdAt"`
		UpdatedAt yamlTime    `yaml:"updatedAt"`
	}
	if err := yaml.Unmarshal(raw, &flexible); err != nil {
		return
	}
	doc.ID = flexible.ID
	doc.Aliases = flexible.Aliases.Values()
	doc.Tags = append(doc.Tags, flexible.Tags.Values()...)
	doc.CreatedAt = flexible.CreatedAt.t
	doc.UpdatedAt = flexible.UpdatedAt.t
}

func parseBody(content []byte, bodyStartLine int, doc *Doc) {
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		lineIdx := bodyStartLine + i
		heading := parseHeading(line, lineIdx)
		if heading != nil {
			doc.Headings = append(doc.Headings, heading)
		}
		block := parseBlockID(line, lineIdx)
		if block != nil {
			doc.Blocks = append(doc.Blocks, block)
		}
		links := parseLinks(line, lineIdx)
		if links != nil {
			doc.Links = append(doc.Links, links...)
		}
	}
}
