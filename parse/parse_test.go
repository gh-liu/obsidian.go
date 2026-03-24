package parse

import (
	"reflect"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		content string
		path    string
		want    *Doc
	}{
		{
			name:    "empty",
			content: "",
			path:    "note.md",
			want:    &Doc{Path: "note.md"},
		},
		{
			name: "frontmatter only",
			content: `---
id: abc-123
title: My Note
aliases: [foo, bar]
tags: [tag1, tag2]
---

`,
			path: "x.md",
			want: &Doc{
				Path:    "x.md",
				ID:      "abc-123",
				Title:   "My Note",
				IDRange: &Range{Start: Pos{1, 0}, End: Pos{1, 11}},
				Aliases: []string{"foo", "bar"},
				Tags:    []string{"tag1", "tag2"},
			},
		},
		{
			name: "frontmatter aliases string",
			content: `---
aliases: single-alias
tags: one-tag
---
`,
			path: "a.md",
			want: &Doc{
				Path:    "a.md",
				Aliases: []string{"single-alias"},
				Tags:    []string{"one-tag"},
			},
		},
		{
			name: "frontmatter createdAt updatedAt",
			content: `---
id: time-doc
createdAt: 2026-02-05
updatedAt: 2026-02-28 18:38:25
---
`,
			path: "time.md",
			want: &Doc{
				Path:      "time.md",
				ID:        "time-doc",
				IDRange:   &Range{Start: Pos{1, 0}, End: Pos{1, 12}},
				CreatedAt: time.Date(2026, 2, 5, 0, 0, 0, 0, time.Local),
				UpdatedAt: time.Date(2026, 2, 28, 18, 38, 25, 0, time.Local),
			},
		},
		{
			name: "headings",
			content: `# Title
## Section 1
### Sub
`,
			path: "h.md",
			want: &Doc{
				Path: "h.md",
				Headings: []*Heading{
					{Level: 1, Text: "Title", Range: Range{Start: Pos{0, 0}, End: Pos{0, 7}}},
					{Level: 2, Text: "Section 1", Range: Range{Start: Pos{1, 0}, End: Pos{1, 12}}},
					{Level: 3, Text: "Sub", Range: Range{Start: Pos{2, 0}, End: Pos{2, 7}}},
				},
			},
		},
		{
			name:    "wiki links",
			content: `See [[note]] and [[other|alias]] and [[file#heading]]`,
			path:    "links.md",
			want: &Doc{
				Path: "links.md",
				Links: []*Link{
					{Kind: LinkWiki, Target: "note", Range: Range{Start: Pos{0, 4}, End: Pos{0, 12}}},
					{Kind: LinkWiki, Target: "other", Alias: "alias", Range: Range{Start: Pos{0, 17}, End: Pos{0, 32}}},
					{Kind: LinkWiki, Target: "file", Anchor: "heading", Range: Range{Start: Pos{0, 37}, End: Pos{0, 53}}},
				},
			},
		},
		{
			name:    "wiki link alias escaped pipe",
			content: `| [[1774318604-FUEZ\|Test-Driven Development: By Example]] |`,
			path:    "table.md",
			want: &Doc{
				Path: "table.md",
				Links: []*Link{
					{Kind: LinkWiki, Target: "1774318604-FUEZ", Alias: "Test-Driven Development: By Example", Range: Range{Start: Pos{0, 2}, End: Pos{0, 58}}},
				},
			},
		},
		{
			name:    "wiki link same-note heading",
			content: `Jump to [[#Section title]]`,
			path:    "same.md",
			want: &Doc{
				Path: "same.md",
				Links: []*Link{
					{Kind: LinkWiki, Target: "", Anchor: "Section title", Range: Range{Start: Pos{0, 8}, End: Pos{0, 26}}},
				},
			},
		},
		{
			name:    "wiki link nested headings",
			content: `[[file#Heading 1#Subheading]]`,
			path:    "nested.md",
			want: &Doc{
				Path: "nested.md",
				Links: []*Link{
					{Kind: LinkWiki, Target: "file", Anchor: "Heading 1#Subheading", Range: Range{Start: Pos{0, 0}, End: Pos{0, 29}}},
				},
			},
		},
		{
			name:    "markdown links",
			content: `[text](path/to/file.md)`,
			path:    "md.md",
			want: &Doc{
				Path: "md.md",
				Links: []*Link{
					{Kind: LinkMarkdown, Target: "path/to/file.md", Alias: "text", Range: Range{Start: Pos{0, 0}, End: Pos{0, 23}}},
				},
			},
		},
		{
			name: "tags from frontmatter only",
			content: `---
tags: [tag1, nested/tag]
---
Content with #tag and #inline here`,
			path: "tags.md",
			want: &Doc{
				Path: "tags.md",
				Tags: []string{"tag1", "nested/tag"},
			},
		},
		{
			name: "block ID in document",
			content: `Some paragraph here ^abc-123
Another line ^block_99`,
			path: "blocks.md",
			want: &Doc{
				Path: "blocks.md",
				Blocks: []*Block{
					{ID: "abc-123", Range: Range{Start: Pos{0, 19}, End: Pos{0, 28}}},
					{ID: "block_99", Range: Range{Start: Pos{1, 12}, End: Pos{1, 22}}},
				},
			},
		},
		{
			name:    "wiki link with block ref",
			content: `[[file#^block-id]]`,
			path:    "block.md",
			want: &Doc{
				Path: "block.md",
				Links: []*Link{
					{Kind: LinkWiki, Target: "file", BlockRef: "block-id", Range: Range{Start: Pos{0, 0}, End: Pos{0, 18}}},
				},
			},
		},
		{
			name:    "wiki link same-note block ref",
			content: `See [[#^my-block]]`,
			path:    "same-block.md",
			want: &Doc{
				Path: "same-block.md",
				Links: []*Link{
					{Kind: LinkWiki, Target: "", BlockRef: "my-block", Range: Range{Start: Pos{0, 4}, End: Pos{0, 18}}},
				},
			},
		},
		{
			name:    "wiki link block ref with alias",
			content: `[[note#^block-id|click here]]`,
			path:    "block-alias.md",
			want: &Doc{
				Path: "block-alias.md",
				Links: []*Link{
					{Kind: LinkWiki, Target: "note", BlockRef: "block-id", Alias: "click here", Range: Range{Start: Pos{0, 0}, End: Pos{0, 29}}},
				},
			},
		},
		{
			name: "full document",
			content: `---
id: doc-1
title: Doc Title
aliases: [a1, a2]
tags: [t1]
---
# Title
See [[other]] and #inline-tag
## Section
`,
			path: "full.md",
			want: &Doc{
				Path:    "full.md",
				ID:      "doc-1",
				Title:   "Doc Title",
				IDRange: &Range{Start: Pos{1, 0}, End: Pos{1, 9}},
				Aliases: []string{"a1", "a2"},
				Tags:    []string{"t1"},
				Headings: []*Heading{
					{Level: 1, Text: "Title", Range: Range{Start: Pos{6, 0}, End: Pos{6, 7}}},
					{Level: 2, Text: "Section", Range: Range{Start: Pos{8, 0}, End: Pos{8, 10}}},
				},
				Links: []*Link{
					{Kind: LinkWiki, Target: "other", Range: Range{Start: Pos{7, 4}, End: Pos{7, 13}}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.content), tt.path)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}
