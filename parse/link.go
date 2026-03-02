package parse

import (
	"regexp"
	"strings"
)

// LinkKind distinguishes wiki links from markdown links.
type LinkKind int

const (
	LinkWiki LinkKind = iota
	LinkMarkdown
)

// Link represents a link to another note or resource.
// Target: at parse, stub Doc with ID=id; at index, resolved Doc. Nil for same-note [[#heading]].
// Anchor: *Heading for #heading (Text=id), *Block for #^block-id (ID=id).
type Link struct {
	Kind   LinkKind
	Target *Doc     // Path holds id at parse; resolved at index; nil = same-note
	Anchor *Heading // for #heading, Text holds id
	Block  *Block   // for #^block-id, ID holds id
	Alias  string   // for wiki [[note|alias]]
	Range  Range
}

// Wiki link: [[file]], [[file|alias]], [[file#heading]], [[file#^block-id]], [[#heading]] (same-note)
// Target can be empty for [[#heading]]; anchor [^\]|]+ stops before | so alias parses correctly
var reWikiLink = regexp.MustCompile(`\[\[([^\]|#]*)(?:#([^\]|]+))?(?:\|([^\]]+))?\]\]`)
var reMDLink = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)

func parseLinks(line string, lineIdx int) (links []*Link) {
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
			if after, ok := strings.CutPrefix(anchorID, "^"); ok {
				block = &Block{ID: after}
			} else {
				anchor = &Heading{Text: anchorID}
			}
		}
		links = append(links, &Link{
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
		links = append(links, &Link{
			Kind:   LinkMarkdown,
			Target: &Doc{ID: targetID},
			Alias:  line[m[2]:m[3]],
			Range: Range{
				Start: Pos{Line: lineIdx, Character: m[0]},
				End:   Pos{Line: lineIdx, Character: m[1]},
			},
		})
	}
	return links
}
