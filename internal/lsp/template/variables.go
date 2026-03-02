package template

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

// Built-in variables (Obsidian Templates plugin compatible):
// {{title}} - note title (from filename)
// {{date}}  - current date (YYYY-MM-DD)
// {{time}}  - current time (HH:mm)
// {{id}}    - unique id (timestamp-XXXX, e.g. 1770123038-LOCB)

// Vars holds variables for template replacement.
type Vars struct {
	Title string // note title (from filename)
	Date  string // 2006-01-02
	Time  string // 15:04
	ID    string // timestamp-XXXX
}

// NewVars returns Args with current date/time, generated id, and the given title.
func NewVars(title string) Vars {
	now := time.Now()
	return Vars{
		Title: title,
		Date:  now.Format("2006-01-02"),
		Time:  now.Format("15:04"),
		ID:    generateID(),
	}
}

// ReplaceAll replaces {{title}}, {{date}}, {{time}}, {{id}} in content with vars.
func (vars Vars) ReplaceAll(content string) string {
	content = strings.ReplaceAll(content, "{{title}}", vars.Title)
	content = strings.ReplaceAll(content, "{{date}}", vars.Date)
	content = strings.ReplaceAll(content, "{{time}}", vars.Time)
	content = strings.ReplaceAll(content, "{{id}}", vars.ID)
	return content
}

// generateID returns a unique id in format timestamp-XXXX (e.g. 1770123038-LOCB).
func generateID() string {
	ts := time.Now().Unix()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// fallback: use timestamp-based pseudo-random
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
