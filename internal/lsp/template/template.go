package template

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultName = "default"

// Built-in variables (Obsidian Templates plugin compatible):
// {{title}} - note title (from filename)
// {{date}}  - current date (YYYY-MM-DD)
// {{time}}  - current time (HH:mm:ss)

// Args holds variables for template replacement.
type Args struct {
	Title string // note title (from filename)
	Date  string // 2006-01-02
	Time  string // 15:04:05
}

// BuiltinDefault returns the built-in default template content.
func BuiltinDefault() string {
	return `---
title: {{title}}
created: {{date}}
---

# {{title}}
`
}

// Load reads template content from templateDir/name.md.
// If not found, returns empty string and nil error (caller should use BuiltinDefault for "default").
func Load(templateDir, name string) (string, error) {
	if name == "" {
		name = defaultName
	}
	path := filepath.Join(templateDir, name+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && name == defaultName {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// Execute replaces {{title}}, {{date}}, {{time}} in content with args.
// Compatible with Obsidian built-in Templates plugin syntax.
func Execute(content string, args Args) string {
	content = strings.ReplaceAll(content, "{{title}}", args.Title)
	content = strings.ReplaceAll(content, "{{date}}", args.Date)
	content = strings.ReplaceAll(content, "{{time}}", args.Time)
	return content
}

// NowArgs returns Args with current date/time and the given title.
func NowArgs(title string) Args {
	now := time.Now()
	return Args{
		Title: title,
		Date:  now.Format("2006-01-02"),
		Time:  now.Format("15:04:05"),
	}
}
