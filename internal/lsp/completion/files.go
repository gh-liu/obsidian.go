package completion

import (
	"crypto/rand"
	"fmt"
	"os"
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

func completeImages(idx *index.Index, prefix string, imagePaths []string) []protocol.CompletionItem {
	var items []protocol.CompletionItem
	prefixLower := strings.ToLower(prefix)
	for _, path := range listImageFiles(idx, imagePaths) {
		if prefixLower != "" && !stringContainsLower(path, prefixLower) && !stringContainsLower(filepath.Base(path), prefixLower) {
			continue
		}
		items = append(items, protocol.CompletionItem{
			Label:      path,
			InsertText: path,
			Kind:       protocol.CompletionItemKindFile,
		})
	}
	sortByRelevance(items, prefixLower)
	return items
}

func listImageFiles(idx *index.Index, imagePaths []string) []string {
	root := idx.Root()
	if len(imagePaths) == 0 {
		imagePaths = []string{""}
	}
	seen := map[string]struct{}{}
	var out []string
	for _, base := range imagePaths {
		base = filepath.Clean(filepath.FromSlash(strings.TrimSpace(base)))
		if base == "." {
			base = ""
		}
		walkRoot := filepath.Join(root, base)
		_ = filepath.Walk(walkRoot, func(fullPath string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() || !isImageFile(info.Name()) {
				return nil
			}
			rel, err := filepath.Rel(root, fullPath)
			if err != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if idx.ShouldIgnore(rel) {
				return nil
			}
			if _, ok := seen[rel]; ok {
				return nil
			}
			seen[rel] = struct{}{}
			out = append(out, rel)
			return nil
		})
	}
	return out
}

func isImageFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".avif", ".bmp":
		return true
	default:
		return false
	}
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

func completeGlobalHeadings(idx *index.Index, prefix string) []protocol.CompletionItem {
	prefixLower := strings.ToLower(prefix)
	var items []protocol.CompletionItem
	for _, entry := range idx.SnapshotPaths() {
		doc := entry.Doc
		if doc == nil {
			continue
		}
		target := linkTargetForDoc(entry.Path, doc.ID)
		for _, h := range doc.Headings {
			if h == nil || (prefixLower != "" && !stringContainsLower(h.Text, prefixLower)) {
				continue
			}
			items = append(items, protocol.CompletionItem{
				Label:      h.Text,
				Detail:     entry.Path,
				InsertText: target + "#" + h.Text,
				Kind:       protocol.CompletionItemKindField,
			})
		}
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
			Detail:     b.Preview,
			Kind:       protocol.CompletionItemKindReference,
		})
	}
	return items
}

func completeGlobalBlocks(idx *index.Index, prefix string) []protocol.CompletionItem {
	prefixLower := strings.ToLower(prefix)
	var items []protocol.CompletionItem
	for _, entry := range idx.SnapshotPaths() {
		doc := entry.Doc
		if doc == nil {
			continue
		}
		target := linkTargetForDoc(entry.Path, doc.ID)
		for _, b := range doc.Blocks {
			if b == nil || (prefixLower != "" && !stringContainsLower(b.ID, prefixLower)) {
				continue
			}
			items = append(items, protocol.CompletionItem{
				Label:      b.ID,
				Detail:     entry.Path + " — " + b.Preview,
				InsertText: target + "#^" + b.ID,
				Kind:       protocol.CompletionItemKindReference,
			})
		}
	}
	return items
}

func linkTargetForDoc(path, id string) string {
	if id != "" {
		return id
	}
	return strings.TrimSuffix(path, ".md")
}

func newBlockIDCompletion() protocol.CompletionItem {
	id := newBlockID()
	return protocol.CompletionItem{
		Label:      "Generate block ID",
		InsertText: id,
		Detail:     "Generate ^" + id,
		Kind:       protocol.CompletionItemKindReference,
	}
}

func newBlockID() string {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "block-id"
	}
	return fmt.Sprintf("%02x%02x%02x", b[0], b[1], b[2])
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
