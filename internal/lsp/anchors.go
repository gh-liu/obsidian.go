package lsp

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/gh-liu/obsidian.go/parse"
)

func headingAnchor(doc *parse.Doc, heading *parse.Heading) string {
	if doc == nil || heading == nil {
		return ""
	}
	anchors := headingAnchors(doc)
	for i, h := range doc.Headings {
		if h == heading {
			return anchors[i]
		}
	}
	return ""
}

func findHeading(doc *parse.Doc, anchor string) *parse.Heading {
	if doc == nil {
		return nil
	}
	want := normalizeHeadingAnchor(anchor)
	if want == "" {
		for _, h := range doc.Headings {
			if h != nil && strings.EqualFold(anchor, h.Text) {
				return h
			}
		}
		return nil
	}
	anchors := headingAnchors(doc)
	for i, h := range doc.Headings {
		if h == nil {
			continue
		}
		if anchors[i] == want {
			return h
		}
	}
	for _, h := range doc.Headings {
		if h != nil && strings.EqualFold(anchor, h.Text) {
			return h
		}
	}
	return nil
}

func headingAtPosition(doc *parse.Doc, lineIdx, byteOff int) *parse.Heading {
	if doc == nil {
		return nil
	}
	for _, h := range doc.Headings {
		if h != nil && inRange(lineIdx, byteOff, h.Range) {
			return h
		}
	}
	return nil
}

func headingAnchors(doc *parse.Doc) []string {
	if doc == nil {
		return nil
	}
	anchors := make([]string, len(doc.Headings))
	seen := make(map[string]int, len(doc.Headings))
	for i, h := range doc.Headings {
		if h == nil {
			continue
		}
		base := normalizeHeadingAnchor(h.Text)
		if base == "" {
			continue
		}
		n := seen[base]
		if n == 0 {
			anchors[i] = base
		} else {
			anchors[i] = base + "-" + strconv.Itoa(n)
		}
		seen[base] = n + 1
	}
	return anchors
}

func normalizeHeadingAnchor(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "\t", "-")
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_':
			b.WriteRune(r)
			lastDash = false
		case r == '-':
			if !lastDash && b.Len() > 0 {
				b.WriteRune(r)
				lastDash = true
			}
		case unicode.IsLetter(r):
			b.WriteRune(r)
			lastDash = false
		}
	}
	return strings.Trim(b.String(), "-")
}
