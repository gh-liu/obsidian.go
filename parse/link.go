package parse

import (
	"regexp"
	"strings"
)

var reMDLink = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)

// parseLinks extracts Obsidian wiki links ([[...]]) and markdown links ([text](url))
// from a single line. The line index is used for Range positions.
func parseLinks(line string, lineIdx int) []*Link {
	var links []*Link

	// Wiki links: [[...]]
	for start := 0; start < len(line)-1; start++ {
		if line[start] != '[' || line[start+1] != '[' {
			continue
		}
		// Skip escaped \[\[
		if start > 0 && line[start-1] == '\\' {
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
		inner := line[start+2 : end]
		target, anchorBlock, alias := parseWikiLinkParts(inner)

		var anchor, blockRef string
		if anchorBlock != "" {
			if after, ok := strings.CutPrefix(anchorBlock, "#^"); ok {
				blockRef = after
			} else {
				anchor = strings.TrimPrefix(anchorBlock, "#")
			}
		}

		links = append(links, &Link{
			Kind:     LinkWiki,
			Target:   target,
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

	// Markdown links: [text](url)
	for _, m := range reMDLink.FindAllStringSubmatchIndex(line, -1) {
		links = append(links, &Link{
			Kind:   LinkMarkdown,
			Target: line[m[4]:m[5]],
			Alias:  line[m[2]:m[3]],
			Range: Range{
				Start: Pos{Line: lineIdx, Character: m[0]},
				End:   Pos{Line: lineIdx, Character: m[1]},
			},
		})
	}

	return links
}
