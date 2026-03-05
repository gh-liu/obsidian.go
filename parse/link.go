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
// Target stores link target id/path. Empty means same-note link like [[#heading]].
// Anchor stores heading anchor text without '#'.
// BlockRef stores block id without '^'.
type Link struct {
	Kind     LinkKind
	Target   string // target path or id; empty for same-note
	Anchor   string // heading text for #heading
	BlockRef string // block id for #^block-id
	Alias    string // for wiki [[note|alias]]
	Range    Range
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
		anchor := ""
		blockRef := ""
		if anchorID != "" {
			if after, ok := strings.CutPrefix(anchorID, "^"); ok {
				blockRef = after
			} else {
				anchor = anchorID
			}
		}
		links = append(links, &Link{
			Kind:     LinkWiki,
			Target:   targetID,
			Anchor:   anchor,
			BlockRef: blockRef,
			Alias:    alias,
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
			Target: targetID,
			Alias:  line[m[2]:m[3]],
			Range: Range{
				Start: Pos{Line: lineIdx, Character: m[0]},
				End:   Pos{Line: lineIdx, Character: m[1]},
			},
		})
	}
	return links
}
