package parse

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// Parse parses a markdown file content into a Doc.
// path is the relative vault path (used for mapping key).
// Character positions in Ranges are UTF-8 byte offsets within each line.
func Parse(content []byte, path string) (*Doc, error) {
	doc := &Doc{Path: path}

	fm, body := splitFrontmatter(content)
	bodyStartLine := 0
	if fm != nil {
		parseFrontmatter(fm, doc, 1) // frontmatter starts at line 1 (0 is the opening ---)
		bodyStartLine = strings.Count(string(content[:len(content)-len(body)]), "\n")
	}
	parseBody(body, bodyStartLine, doc)

	return doc, nil
}

// splitFrontmatter splits YAML frontmatter from the body.
// Content must start with "---\n". Returns (frontmatter_bytes, body_bytes).
// frontmatter is nil if the content does not start with "---\n".
func splitFrontmatter(content []byte) (frontmatter, body []byte) {
	s := string(content)
	if !strings.HasPrefix(s, "---\n") {
		return nil, content
	}
	before, after, ok := strings.Cut(s[4:], "\n---")
	if !ok {
		// Closing --- not found; treat as no frontmatter.
		return nil, content
	}
	return []byte(before), []byte(after)
}

// parseFrontmatter unmarshals YAML frontmatter into Doc fields.
// startLine is the 0-based line index of the first frontmatter line (after opening ---).
func parseFrontmatter(raw []byte, doc *Doc, startLine int) {
	var fm struct {
		ID        string      `yaml:"id"`
		Title     string      `yaml:"title"`
		Aliases   yamlStrings `yaml:"aliases"`
		Tags      yamlStrings `yaml:"tags"`
		CreatedAt yamlTime    `yaml:"createdAt"`
		UpdatedAt yamlTime    `yaml:"updatedAt"`
	}
	if err := yaml.Unmarshal(raw, &fm); err != nil {
		return
	}
	doc.ID = fm.ID
	doc.Title = strings.TrimSpace(fm.Title)
	doc.Aliases = fm.Aliases.Values()
	doc.Tags = fm.Tags.Values()
	doc.CreatedAt = fm.CreatedAt.t
	doc.UpdatedAt = fm.UpdatedAt.t
}

// parseBody processes body content line by line, extracting headings, blocks, and links.
func parseBody(content []byte, bodyStartLine int, doc *Doc) {
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		lineIdx := bodyStartLine + i

		if h := parseHeading(line, lineIdx); h != nil {
			doc.Headings = append(doc.Headings, h)
		}
		if b := parseBlockID(line, lineIdx); b != nil {
			doc.Blocks = append(doc.Blocks, b)
		}
		if l := parseLinks(line, lineIdx); l != nil {
			doc.Links = append(doc.Links, l...)
		}
	}
}
