package completion

import (
	"path/filepath"
	"strings"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/protocol"
)

func completeFiles(idx *index.Index, prefix string) []protocol.CompletionItem {
	var items []protocol.CompletionItem
	prefixLower := strings.ToLower(prefix)

	for _, entry := range idx.SnapshotPaths() {
		path := entry.Path
		doc := entry.Doc
		if doc == nil {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(path), ".md")

		// Check match against: basename, title, id, and all aliases.
		if prefixLower != "" {
			matched := stringContainsLower(name, prefixLower) ||
				stringContainsLower(doc.Title, prefixLower) ||
				stringContainsLower(doc.ID, prefixLower)
			for _, a := range doc.Aliases {
				if stringContainsLower(a, prefixLower) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Label: prefer title, fallback to name.
		label := name
		detail := ""
		if doc.Title != "" && !strings.EqualFold(doc.Title, name) {
			label = doc.Title
			detail = path
		}

		// InsertText: [[id|title]] format when both available,
		// otherwise [[id]], falling back to path.
		insertText := path
		if doc.ID != "" {
			insertText = doc.ID
			if doc.Title != "" && !strings.EqualFold(doc.ID, doc.Title) {
				insertText = doc.ID + "|" + doc.Title
			}
		}

		item := protocol.CompletionItem{
			Label:      label,
			Detail:     detail,
			InsertText: insertText,
			Kind:       protocol.CompletionItemKindFile,
		}
		items = append(items, item)
	}

	// Sort by relevance
	sortByRelevance(items, prefixLower)
	return items
}

func completeHeadings(idx *index.Index, targetPath, currentRel, prefix string) []protocol.CompletionItem {
	resolvedPath := currentRel
	if targetPath != "" {
		resolvedPath = idx.ResolveLinkTargetToPath(targetPath)
		if resolvedPath == "" {
			return nil
		}
	}

	doc := idx.GetByPath(resolvedPath)
	if doc == nil {
		return nil
	}

	prefixLower := strings.ToLower(prefix)
	var items []protocol.CompletionItem
	for _, h := range doc.Headings {
		if h == nil {
			continue
		}
		if prefixLower != "" && !stringContainsLower(h.Text, prefixLower) {
			continue
		}
		items = append(items, protocol.CompletionItem{
			Label:      h.Text,
			InsertText: h.Text,
			Kind:       protocol.CompletionItemKindField,
		})
	}
	return items
}

func completeBlocks(idx *index.Index, targetPath, currentRel, prefix string) []protocol.CompletionItem {
	resolvedPath := currentRel
	if targetPath != "" {
		resolvedPath = idx.ResolveLinkTargetToPath(targetPath)
		if resolvedPath == "" {
			return nil
		}
	}

	doc := idx.GetByPath(resolvedPath)
	if doc == nil {
		return nil
	}

	prefixLower := strings.ToLower(prefix)
	var items []protocol.CompletionItem
	for _, b := range doc.Blocks {
		if b == nil {
			continue
		}
		if prefixLower != "" && !stringContainsLower(b.ID, prefixLower) {
			continue
		}
		items = append(items, protocol.CompletionItem{
			Label:      b.ID,
			InsertText: b.ID,
			Kind:       protocol.CompletionItemKindReference,
		})
	}
	return items
}

func completeAliases(idx *index.Index, targetPath, prefix string) []protocol.CompletionItem {
	if targetPath == "" {
		return nil
	}
	resolvedPath := idx.ResolveLinkTargetToPath(targetPath)
	if resolvedPath == "" {
		return nil
	}
	doc := idx.GetByPath(resolvedPath)
	if doc == nil {
		return nil
	}

	prefixLower := strings.ToLower(prefix)
	var items []protocol.CompletionItem
	for _, a := range doc.Aliases {
		if prefixLower != "" && !stringContainsLower(a, prefixLower) {
			continue
		}
		items = append(items, protocol.CompletionItem{
			Label:      a,
			InsertText: a,
			Kind:       protocol.CompletionItemKindText,
		})
	}
	return items
}
