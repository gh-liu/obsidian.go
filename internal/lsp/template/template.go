package template

import (
	"os"
	"path/filepath"
	"strings"
	"time"
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

// EnsureFrontmatterDefaults ensures content has frontmatter with default template fields
// (id, title, createdAt, updatedAt). If no frontmatter, prepends full block; if present, injects missing fields.
// updatedAt is always set to current time when formatting.
// title is typically the note filename without .md.
func EnsureFrontmatterDefaults(content, title string) string {
	vars := NewVars(title)
	updatedAt := time.Now().Format("2006-01-02 15:04:05")
	if !strings.HasPrefix(content, "---\n") {
		return frontmatterBlock(vars, updatedAt) + "\n" + content
	}
	idx := strings.Index(content[4:], "\n---")
	if idx < 0 {
		return content
	}
	fm := content[4 : 4+idx]
	rest := content[4+idx:]
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
	if len(inject) == 0 && fm == content[4:4+idx] {
		return content
	}
	newFm := strings.Join(inject, "\n")
	if newFm != "" {
		newFm += "\n"
	}
	newFm += fm
	return content[:4] + newFm + rest
}

func replaceKeyValue(fm, key, value string) string {
	prefix := key + ":"
	var out []string
	replaced := false
	for _, line := range strings.Split(fm, "\n") {
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

func frontmatterBlock(v Vars, updatedAt string) string {
	return "---\nid: " + v.ID + "\ntitle: " + v.Title + "\ncreatedAt: " + v.Date + "\nupdatedAt: " + updatedAt + "\n---"
}
