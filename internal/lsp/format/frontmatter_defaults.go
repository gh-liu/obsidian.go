package format

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

// EnsureFrontmatterDefaults ensures content has frontmatter with default fields:
// id, title, createdAt, updatedAt. updatedAt is always refreshed.
func EnsureFrontmatterDefaults(content, title string) string {
	vars := newFrontmatterVars(title)
	updatedAt := time.Now().Format("2006-01-02 15:04:05")
	if !strings.HasPrefix(content, "---\n") {
		return frontmatterBlock(vars, updatedAt) + "\n" + content
	}
	fm, after, ok := strings.Cut(content[4:], "\n---")
	if !ok {
		return content
	}
	origFm := fm
	var inject []string
	if !hasKey(fm, "id") {
		inject = append(inject, "id: "+vars.ID)
	}
	if !hasKey(fm, "title") {
		inject = append(inject, "title: "+title)
	}
	if !hasKey(fm, "createdAt") {
		inject = append(inject, "createdAt: "+vars.Date)
	}
	if !hasKey(fm, "updatedAt") {
		inject = append(inject, "updatedAt: "+updatedAt)
	} else {
		fm = replaceKeyValue(fm, "updatedAt", updatedAt)
	}
	if len(inject) == 0 && fm == origFm {
		return content
	}
	newFm := strings.Join(inject, "\n")
	if newFm != "" {
		newFm += "\n"
	}
	newFm += fm
	return content[:4] + newFm + "\n---" + after
}

type frontmatterVars struct {
	Title string
	Date  string
	ID    string
}

func newFrontmatterVars(title string) frontmatterVars {
	now := time.Now()
	return frontmatterVars{
		Title: title,
		Date:  now.Format("2006-01-02"),
		ID:    generateID(),
	}
}

func hasKey(fm, key string) bool {
	prefix := key + ":"
	for line := range strings.SplitSeq(fm, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func replaceKeyValue(fm, key, value string) string {
	prefix := key + ":"
	var out []string
	replaced := false
	for line := range strings.SplitSeq(fm, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			out = append(out, key+": "+value)
			replaced = true
		} else {
			out = append(out, line)
		}
	}
	if !replaced {
		return fm
	}
	return strings.Join(out, "\n")
}

func frontmatterBlock(v frontmatterVars, updatedAt string) string {
	return "---\nid: " + v.ID + "\ntitle: " + v.Title + "\ncreatedAt: " + v.Date + "\nupdatedAt: " + updatedAt + "\n---"
}

func generateID() string {
	ts := time.Now().Unix()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		b[0] = byte(ts >> 0)
		b[1] = byte(ts >> 8)
		b[2] = byte(ts >> 16)
		b[3] = byte(ts >> 24)
	}
	for i := range b {
		b[i] = 'A' + (b[i] % 26)
	}
	return fmt.Sprintf("%d-%s", ts, string(b))
}
