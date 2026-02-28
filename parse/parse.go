package parse

import (
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

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
	idx := strings.Index(s[4:], "\n---")
	if idx < 0 {
		return nil, content
	}
	end := 4 + idx + 4 // after second ---
	return []byte(s[4 : 4+idx]), []byte(s[end:])
}

var timeLayouts = []string{
	"2006-01-02 15:04:05", // updatedAt: 2026-02-28 18:38:25
	"2006-01-02",          // createdAt: 2026-02-05
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

func parseFrontmatter(raw []byte, doc *Doc) {
	// Obsidian: aliases/tags can be string or array
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

type yamlAliases struct {
	values []string
}

func (y *yamlAliases) UnmarshalYAML(n *yaml.Node) error {
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

func (y *yamlAliases) Values() []string {
	if y == nil {
		return nil
	}
	return y.values
}

type yamlTags struct {
	values []string
}

func (y *yamlTags) UnmarshalYAML(n *yaml.Node) error {
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

func (y *yamlTags) Values() []string {
	if y == nil {
		return nil
	}
	return y.values
}

var (
	reHeading = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	// Wiki link: [[file]], [[file|alias]], [[file#heading]], [[file#^block-id]], [[#heading]] (same-note)
	// Target can be empty for [[#heading]]; anchor [^\]|]+ stops before | so alias parses correctly
	reWikiLink = regexp.MustCompile(`\[\[([^\]|#]*)(?:#([^\]|]+))?(?:\|([^\]]+))?\]\]`)
	reMDLink   = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
)

func parseBody(content []byte, bodyStartLine int, doc *Doc) {
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		lineIdx := bodyStartLine + i
		parseHeading(line, lineIdx, doc)
		parseLinks(line, lineIdx, doc)
	}
}

func parseHeading(line string, lineIdx int, doc *Doc) {
	m := reHeading.FindStringSubmatch(line)
	if m == nil {
		return
	}
	doc.Headings = append(doc.Headings, Heading{
		Level: len(m[1]),
		Text:  strings.TrimSpace(m[2]),
		Range: Range{
			Start: Pos{Line: lineIdx, Character: 0},
			End:   Pos{Line: lineIdx, Character: len(line)},
		},
	})
}

func parseLinks(line string, lineIdx int, doc *Doc) {
	for _, m := range reWikiLink.FindAllStringSubmatchIndex(line, -1) {
		targetID := line[m[2]:m[3]]
		anchorID, alias := "", ""
		if m[4] >= 0 {
			anchorID = line[m[4]:m[5]]
		}
		if m[6] >= 0 {
			alias = line[m[6]:m[7]]
		}
		var target *Doc
		if targetID != "" {
			target = &Doc{ID: targetID}
		}
		var anchor *Heading
		var block *Block
		if anchorID != "" {
			if strings.HasPrefix(anchorID, "^") {
				block = &Block{ID: strings.TrimPrefix(anchorID, "^")}
			} else {
				anchor = &Heading{Text: anchorID}
			}
		}
		doc.Links = append(doc.Links, Link{
			Kind:   LinkWiki,
			Target: target,
			Anchor: anchor,
			Block:  block,
			Alias:  alias,
			Range: Range{
				Start: Pos{Line: lineIdx, Character: m[0]},
				End:   Pos{Line: lineIdx, Character: m[1]},
			},
		})
	}
	for _, m := range reMDLink.FindAllStringSubmatchIndex(line, -1) {
		targetID := line[m[4]:m[5]]
		doc.Links = append(doc.Links, Link{
			Kind:   LinkMarkdown,
			Target: &Doc{ID: targetID},
			Alias:  line[m[2]:m[3]],
			Range: Range{
				Start: Pos{Line: lineIdx, Character: m[0]},
				End:   Pos{Line: lineIdx, Character: m[1]},
			},
		})
	}
}
