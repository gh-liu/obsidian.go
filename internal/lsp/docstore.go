package lsp

import (
	"path/filepath"
	"strings"
	"sync"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// DocStore holds open document content for completion and other features
// that need unsaved/in-memory state. Thread-safe.
type DocStore struct {
	mu   sync.RWMutex
	docs map[string]string // URI string -> content
}

func newDocStore() *DocStore {
	return &DocStore{docs: make(map[string]string)}
}

func (s *DocStore) put(uri protocol.DocumentURI, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docs[string(uri)] = content
}

func (s *DocStore) get(uri protocol.DocumentURI) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.docs[string(uri)]
}

func (s *DocStore) remove(uri protocol.DocumentURI) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, string(uri))
}

// LineAt returns the line content at 0-based line index, or empty string.
func (s *DocStore) lineAt(uri protocol.DocumentURI, line int) string {
	content := s.get(uri)
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}
	return lines[line]
}

// Lines returns all lines of the document.
func (s *DocStore) lines(uri protocol.DocumentURI) []string {
	content := s.get(uri)
	return strings.Split(content, "\n")
}

// applyChanges applies content changes to the given document. Full sync if any change has no Range.
func (s *DocStore) applyChanges(uri protocol.DocumentURI, changes []protocol.TextDocumentContentChangeEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	content := s.docs[string(uri)]
	for _, c := range changes {
		// Full sync: client omits range, we get zero value
		if c.Range.Start.Line == 0 && c.Range.Start.Character == 0 && c.Range.End.Line == 0 && c.Range.End.Character == 0 {
			content = c.Text
			continue
		}
		// Incremental: replace Range with Text
		lines := strings.Split(content, "\n")
		startLine := int(c.Range.Start.Line)
		endLine := int(c.Range.End.Line)
		startChar := int(c.Range.Start.Character)
		endChar := int(c.Range.End.Character)
		if startLine < 0 || startLine >= len(lines) {
			continue
		}
		if endLine >= len(lines) {
			endLine = len(lines) - 1
		}
		var before, after string
		before = strings.Join(lines[:startLine], "\n")
		if startLine > 0 {
			before += "\n"
		}
		before += lines[startLine][:min(startChar, len(lines[startLine]))]
		if endLine < len(lines) {
			after = lines[endLine][min(endChar, len(lines[endLine])):]
			if endLine+1 < len(lines) {
				after += "\n" + strings.Join(lines[endLine+1:], "\n")
			}
		}
		content = before + c.Text + after
	}
	s.docs[string(uri)] = content
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// uriToRelPath converts document URI to path relative to root. Returns empty if outside root.
func uriToRelPath(docURI protocol.DocumentURI, root string) string {
	fullPath := uri.URI(docURI).Filename()
	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "..") {
		return ""
	}
	return rel
}
