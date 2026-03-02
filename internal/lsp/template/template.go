package template

import (
	"os"
	"path/filepath"
	"strings"
)

const defaultName = "default"

const defaultTemplate = `---
id: {{id}}
title: {{title}}
created: {{date}}
---

# {{title}}
`

type Template struct {
	path    string
	Content string
}

// Load load template from templateDir/name.md.
func Load(templateDir, name string) (*Template, error) {
	if name == "" {
		name = defaultName
	}
	path := filepath.Join(templateDir, name+".md")
	temp := Template{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && name == defaultName {
			temp.Content = defaultTemplate
			return &temp, nil
		}
		return nil, err
	}
	temp.Content = string(data)
	return &temp, nil
}

// Execute replaces vars
// Compatible with Obsidian built-in Templates plugin syntax.
// Notes created from template always have id in frontmatter; if missing, it is injected.
func (tmp *Template) Execute(vars Vars) string {
	content := vars.ReplaceAll(tmp.Content)
	return ensureFrontmatterID(content, vars.ID)
}

// ensureFrontmatterID injects id into frontmatter if not present. Notes from template must have id.
func ensureFrontmatterID(content, id string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	idx := strings.Index(content[4:], "\n---")
	if idx < 0 {
		return content
	}
	fm := content[4 : 4+idx]
	if hasIDKey(fm) {
		return content
	}
	return content[:4] + "id: " + id + "\n" + fm + content[4+idx:]
}

func hasIDKey(fm string) bool {
	for line := range strings.SplitSeq(fm, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "id:") {
			return true
		}
	}
	return false
}
