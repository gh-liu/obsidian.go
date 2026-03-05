package template

import (
	"os"
	"path/filepath"
	"strings"
)

const DefaultName = "default"

const defaultTemplate = `---
id: {{id}}
title: {{title}}
createdAt: {{date}}
---

# {{title}}
`

type Template struct {
	path    string
	Content string
}

// ListNames returns template names (basenames without .md) in templateDir.
// Returns nil slice if dir does not exist or is empty.
func ListNames(templateDir string) ([]string, error) {
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		base := e.Name()
		lower := strings.ToLower(base)
		if strings.HasSuffix(lower, ".md") {
			names = append(names, base[:len(base)-3])
		}
	}
	return names, nil
}

// Load load template from templateDir/name.md.
func Load(templateDir, name string) (*Template, error) {
	if name == "" {
		name = DefaultName
	}
	path := filepath.Join(templateDir, name+".md")
	temp := Template{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && name == DefaultName {
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
	before, after, ok := strings.Cut(content[4:], "\n---")
	if !ok {
		return content
	}
	if hasIDKey(before) {
		return content
	}
	return content[:4] + "id: " + id + "\n" + before + "\n---" + after
}

func hasIDKey(fm string) bool {
	return hasKey(fm, "id")
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

