package parse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testdataDir = "testdata"

func TestParseTestdataFiles(t *testing.T) {
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	files := 0
	withFrontmatter := 0
	withID := 0
	totalLinks := 0
	totalHeadings := 0
	totalBlocks := 0

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		fullPath := filepath.Join(testdataDir, e.Name())
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("read %s: %v", e.Name(), err)
			continue
		}
		doc, err := Parse(content, e.Name())
		if err != nil {
			t.Errorf("parse %s: %v", e.Name(), err)
			continue
		}
		files++
		if doc.ID != "" || doc.Title != "" || len(doc.Tags) > 0 || len(doc.Aliases) > 0 {
			withFrontmatter++
		}
		if doc.ID != "" {
			withID++
		}
		totalLinks += len(doc.Links)
		totalHeadings += len(doc.Headings)
		totalBlocks += len(doc.Blocks)

		t.Logf("  %s: id=%q title=%q h=%d l=%d b=%d",
			e.Name(), doc.ID, doc.Title, len(doc.Headings), len(doc.Links), len(doc.Blocks))
	}

	t.Logf("files=%d frontmatter=%d id=%d links=%d headings=%d blocks=%d",
		files, withFrontmatter, withID, totalLinks, totalHeadings, totalBlocks)

	if files == 0 {
		t.Fatal("no testdata files parsed")
	}
}

func TestParseFullFrontmatter(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(testdataDir, "full_frontmatter.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	doc, err := Parse(content, "full_frontmatter.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// ID
	if doc.ID != "1774849813-NCZI" {
		t.Errorf("ID = %q, want %q", doc.ID, "1774849813-NCZI")
	}
	// Title
	if doc.Title != "DOD" {
		t.Errorf("Title = %q, want %q", doc.Title, "DOD")
	}
	// Tags
	if len(doc.Tags) != 1 || doc.Tags[0] != "BN" {
		t.Errorf("Tags = %v, want [BN]", doc.Tags)
	}
	// Aliases
	if len(doc.Aliases) != 2 || doc.Aliases[0] != "DOD" || doc.Aliases[1] != "Data-Oriented Design" {
		t.Errorf("Aliases = %v, want [DOD Data-Oriented Design]", doc.Aliases)
	}
	// Dates
	if doc.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
	if doc.UpdatedAt.IsZero() {
		t.Error("UpdatedAt is zero")
	}
	// Headings
	if len(doc.Headings) != 6 {
		t.Errorf("Headings = %d, want 6", len(doc.Headings))
	}
	if doc.Headings[0].Level != 1 || doc.Headings[0].Text != "The Art of Data-Oriented Design" {
		t.Errorf("Heading[0] = H%d %q", doc.Headings[0].Level, doc.Headings[0].Text)
	}
	// No links in this file
	if len(doc.Links) != 0 {
		t.Errorf("Links = %d, want 0", len(doc.Links))
	}
}

func TestParseWikiLinks(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(testdataDir, "wiki_links.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	doc, err := Parse(content, "wiki_links.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Should have frontmatter
	if doc.ID != "1770123039-URZR" {
		t.Errorf("ID = %q", doc.ID)
	}
	if len(doc.Tags) != 2 {
		t.Errorf("Tags = %v, want [CS GO]", doc.Tags)
	}

	// Count wiki links vs markdown links
	wikiCount := 0
	mdCount := 0
	for _, l := range doc.Links {
		switch l.Kind {
		case LinkWiki:
			wikiCount++
		case LinkMarkdown:
			mdCount++
		}
		t.Logf("  Link: kind=%d target=%q anchor=%q blockRef=%q alias=%q line=%d",
			l.Kind, l.Target, l.Anchor, l.BlockRef, l.Alias, l.Range.Start.Line)
	}

	// Expected: 4 wiki links + 1 markdown link (from file content)
	// [[1774849813-NCZI|DOD]], [[1773826925-KAJG|observability]],
	// [[#sudog---waiting-list|sudog]], [[schedule-details]],
	// [[1773846392-UKNM|kafka]] + [Go scheduler paper](...)
	if wikiCount < 4 {
		t.Errorf("wiki links = %d, want >= 4", wikiCount)
	}
	if mdCount < 1 {
		t.Errorf("markdown links = %d, want >= 1", mdCount)
	}

	// Check specific link: [[#sudog---waiting-list|sudog]]
	foundSameNoteAnchor := false
	for _, l := range doc.Links {
		if l.Target == "" && l.Anchor == "sudog---waiting-list" && l.Alias == "sudog" {
			foundSameNoteAnchor = true
		}
	}
	if !foundSameNoteAnchor {
		t.Error("did not find [[#sudog---waiting-list|sudog]]")
	}
}

func TestParseNoFrontmatter(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(testdataDir, "no_frontmatter.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	doc, err := Parse(content, "no_frontmatter.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if doc.ID != "" || doc.Title != "" {
		t.Error("expected no frontmatter fields")
	}
	if len(doc.Headings) < 4 {
		t.Errorf("headings = %d, want >= 4", len(doc.Headings))
	}
	if len(doc.Links) < 6 {
		t.Errorf("links = %d, want >= 6", len(doc.Links))
	}

	// Check block IDs
	if len(doc.Blocks) != 3 {
		t.Errorf("blocks = %d, want 3", len(doc.Blocks))
	}
	blockIDs := make(map[string]bool)
	for _, b := range doc.Blocks {
		blockIDs[b.ID] = true
	}
	if !blockIDs["sec2-anchor"] {
		t.Error("missing block 'sec2-anchor'")
	}
	if !blockIDs["inline-block"] {
		t.Error("missing block 'inline-block'")
	}
	if !blockIDs["nested-block"] {
		t.Error("missing block 'nested-block'")
	}

	// Verify [[target#^sec2-anchor]] link
	foundBlockRef := false
	for _, l := range doc.Links {
		if l.Target == "target" && l.BlockRef == "sec2-anchor" {
			foundBlockRef = true
		}
	}
	if !foundBlockRef {
		t.Error("did not find [[target#^sec2-anchor]]")
	}
}

func TestParseMinimalFrontmatter(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(testdataDir, "minimal_frontmatter.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	doc, err := Parse(content, "minimal_frontmatter.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if doc.ID != "minimal" {
		t.Errorf("ID = %q, want %q", doc.ID, "minimal")
	}
	if doc.Title != "Minimal" {
		t.Errorf("Title = %q, want %q", doc.Title, "Minimal")
	}
	if len(doc.Tags) != 0 {
		t.Errorf("Tags = %v, want empty", doc.Tags)
	}
	if len(doc.Aliases) != 0 {
		t.Errorf("Aliases = %v, want empty", doc.Aliases)
	}
	if len(doc.Headings) != 2 {
		t.Errorf("headings = %d, want 2", len(doc.Headings))
	}
	if len(doc.Blocks) != 1 || doc.Blocks[0].ID != "block-at-start" {
		t.Errorf("blocks = %v", doc.Blocks)
	}
}

func TestParseHeadingAnchorLink(t *testing.T) {
	content := `---
id: test-123
title: Test
tags: [X]
---
# Top Heading

Some text with [[#sub-heading]] and [[other|alias-name]].

## Sub Heading
Content here.

## Another
More [[test-456]] links.
`
	doc, err := Parse([]byte(content), "test.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(doc.Headings) != 3 {
		t.Errorf("headings = %d, want 3", len(doc.Headings))
	}
	if len(doc.Links) != 3 {
		t.Fatalf("links = %d, want 3", len(doc.Links))
	}

	// [[#sub-heading]] → Target="" Anchor="sub-heading"
	l1 := doc.Links[0]
	if l1.Target != "" || l1.Anchor != "sub-heading" {
		t.Errorf("link[0]: target=%q anchor=%q, want target=\"\" anchor=\"sub-heading\"", l1.Target, l1.Anchor)
	}

	// [[other|alias-name]] → Target="other" Alias="alias-name"
	l2 := doc.Links[1]
	if l2.Target != "other" || l2.Alias != "alias-name" {
		t.Errorf("link[1]: target=%q alias=%q, want target=\"other\" alias=\"alias-name\"", l2.Target, l2.Alias)
	}

	// [[test-456]] → Target="test-456"
	l3 := doc.Links[2]
	if l3.Target != "test-456" {
		t.Errorf("link[2]: target=%q, want \"test-456\"", l3.Target)
	}
}

func TestParseFrontmatterVariants(t *testing.T) {
	// String tags (single value)
	content := `---
id: abc
tags: single-tag
---
# Test
`
	doc, err := Parse([]byte(content), "test.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Tags) != 1 || doc.Tags[0] != "single-tag" {
		t.Errorf("tags = %v, want [single-tag]", doc.Tags)
	}

	// ID and title as quoted strings
	content2 := `---
id: "12345"
title: "Hello"
---
`
	doc2, err := Parse([]byte(content2), "test2.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc2.ID != "12345" {
		t.Errorf("id = %q, want \"12345\"", doc2.ID)
	}
	if doc2.Title != "Hello" {
		t.Errorf("title = %q, want \"Hello\"", doc2.Title)
	}
}

func TestParseNoFrontmatterInline(t *testing.T) {
	content := `# Just a heading

Some text [[link]] here.
`
	doc, err := Parse([]byte(content), "nofm.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.ID != "" || doc.Title != "" {
		t.Error("expected no frontmatter fields")
	}
	if len(doc.Headings) != 1 {
		t.Errorf("headings = %d, want 1", len(doc.Headings))
	}
	if len(doc.Links) != 1 || doc.Links[0].Target != "link" {
		t.Errorf("links = %v", doc.Links)
	}
}

func TestParseBlockID(t *testing.T) {
	content := []byte(`# Test

Some text ^my-block-id

More text ^another
`)
	doc, err := Parse(content, "test.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(doc.Blocks))
	}
	if doc.Blocks[0].ID != "my-block-id" {
		t.Errorf("block[0].ID = %q, want \"my-block-id\"", doc.Blocks[0].ID)
	}
	if doc.Blocks[1].ID != "another" {
		t.Errorf("block[1].ID = %q, want \"another\"", doc.Blocks[1].ID)
	}
}

func TestParseStructuredBlockPreviewRequiresBlankLines(t *testing.T) {
	content := []byte(`# Test

> quote line one
> quote line two

^quote-block

After quote

| A | B |
| - | - |
| 1 | 2 |
^not-structured-without-blank-before
`)
	doc, err := Parse(content, "test.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1: %#v", len(doc.Blocks), doc.Blocks)
	}
	block := doc.Blocks[0]
	if block.ID != "quote-block" {
		t.Fatalf("block ID = %q, want quote-block", block.ID)
	}
	wantPreview := "> quote line one\n> quote line two"
	if block.Preview != wantPreview {
		t.Fatalf("preview = %q, want %q", block.Preview, wantPreview)
	}
}

func TestParseCodeBlockFenceSkipsContent(t *testing.T) {
	// # lines inside fenced code blocks must not be parsed as headings.
	content := "" +
		"# Real Heading\n" +
		"\n" +
		"```bash\n" +
		"#!/bin/bash\n" +
		"# this is a bash comment\n" +
		"echo hello\n" +
		"```\n" +
		"\n" +
		"## Another Heading\n"

	doc, err := Parse([]byte(content), "test.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(doc.Headings) != 2 {
		t.Errorf("headings = %d, want 2", len(doc.Headings))
		for _, h := range doc.Headings {
			t.Logf("  heading: H%d %q (line %d)", h.Level, h.Text, h.Range.Start.Line)
		}
	}
	if doc.Headings[0].Text != "Real Heading" {
		t.Errorf("heading[0] = %q, want 'Real Heading'", doc.Headings[0].Text)
	}
	if doc.Headings[1].Text != "Another Heading" {
		t.Errorf("heading[1] = %q, want 'Another Heading'", doc.Headings[1].Text)
	}

	// Links inside code blocks should also be skipped.
	content2 := "" +
		"# Top\n" +
		"\n" +
		"```markdown\n" +
		"[[not-a-real-link]]\n" +
		"```\n"

	doc2, err := Parse([]byte(content2), "test2.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc2.Links) != 0 {
		t.Errorf("links inside code block = %d, want 0", len(doc2.Links))
	}

	// Tilde fences should also work.
	content3 := "" +
		"# Top\n" +
		"\n" +
		"~~~bash\n" +
		"# not a heading\n" +
		"~~~\n"

	doc3, err := Parse([]byte(content3), "test3.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc3.Headings) != 1 {
		t.Errorf("headings inside ~~~ fence = %d, want 1", len(doc3.Headings))
	}

	// Cross-fence: ``` inside ~~~ should not close the fence.
	content4 := "" +
		"# Top\n" +
		"\n" +
		"~~~\n" +
		"```\n" +
		"# still inside fence\n" +
		"~~~\n" +
		"\n" +
		"## Real Again\n"

	doc4, err := Parse([]byte(content4), "test4.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc4.Headings) != 2 {
		t.Errorf("cross-fence headings = %d, want 2", len(doc4.Headings))
		for _, h := range doc4.Headings {
			t.Logf("  heading: H%d %q (line %d)", h.Level, h.Text, h.Range.Start.Line)
		}
	}
}

func TestWikiLinkCursorContext(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		byteOff    int
		wantFiles  bool
		wantBlocks bool
		wantAlias  bool
		wantTarget string
	}{
		{"file completion", "[[", 2, true, false, false, ""},
		{"file completion prefix", "[[note", 5, true, false, false, ""},
		{"heading completion", "[[#", 3, false, false, false, ""},
		{"heading completion prefix", "[[#my-he", 7, false, false, false, ""},
		{"cross heading", "[[file#head", 10, false, false, false, "file"},
		{"block completion", "[[file#^", 8, false, true, false, "file"},
		{"alias completion", "[[file|", 7, false, false, true, "file"},
		{"alias completion prefix", "[[file|al", 9, false, false, true, "file"},
		{"complete link past end", "[[done]]", 8, false, false, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ParseWikiLinkCursorContext(tt.line, tt.byteOff)
			if tt.wantFiles && ctx == nil {
				t.Fatal("expected non-nil context")
			}
			if !tt.wantFiles && !tt.wantBlocks && !tt.wantAlias && ctx == nil {
				return
			}
			if ctx == nil {
				t.Fatal("unexpected nil context")
			}
			if ctx.CompleteFiles != tt.wantFiles {
				t.Errorf("CompleteFiles = %v, want %v", ctx.CompleteFiles, tt.wantFiles)
			}
			if ctx.CompleteBlock != tt.wantBlocks {
				t.Errorf("CompleteBlock = %v, want %v", ctx.CompleteBlock, tt.wantBlocks)
			}
			if ctx.CompleteAlias != tt.wantAlias {
				t.Errorf("CompleteAlias = %v, want %v", ctx.CompleteAlias, tt.wantAlias)
			}
			if tt.wantTarget != "" && ctx.TargetPath != tt.wantTarget {
				t.Errorf("TargetPath = %q, want %q", ctx.TargetPath, tt.wantTarget)
			}
		})
	}
}
