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

var reMDLink = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)

func parseLinks(line string, lineIdx int) (links []*Link) {
	for start := 0; start < len(line)-1; start++ {
		if line[start] != '[' || line[start+1] != '[' {
			continue
		}
		end := -1
		for i := start + 2; i < len(line)-1; i++ {
			if line[i] == '\\' && i+1 < len(line) {
				i++
				continue
			}
			if line[i] == ']' && line[i+1] == ']' {
				end = i
				break
			}
		}
		if end < 0 {
			continue
		}

		targetID, anchorID, alias := parseWikiLinkParts(line[start+2 : end])
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
				Start: Pos{Line: lineIdx, Character: start},
				End:   Pos{Line: lineIdx, Character: end + 2},
			},
		})
		start = end + 1
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
