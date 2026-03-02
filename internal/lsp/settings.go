package lsp

import (
	"path/filepath"
	"regexp"
	"sync"
)

// Settings holds LSP server settings.
type Settings struct {
	mu             sync.RWMutex
	ignorePatterns []*regexp.Regexp
	templatePath   string // relative to vault root, default ".templates"
}

// SetTemplatePath sets the template directory path (relative to vault root).
func (s *Settings) SetTemplatePath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if path == "" {
		s.templatePath = ".templates"
		return
	}
	s.templatePath = filepath.Clean(path)
}

// TemplatePath returns the template directory path.
func (s *Settings) TemplatePath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.templatePath == "" {
		return ".templates"
	}
	return s.templatePath
}

// SetIgnorePatterns sets ignore regex patterns. Invalid patterns are skipped.
func (s *Settings) SetIgnorePatterns(patterns []string) {
	var compiled []*regexp.Regexp
	for _, p := range patterns {
		if p == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		compiled = append(compiled, re)
	}
	s.mu.Lock()
	s.ignorePatterns = compiled
	s.mu.Unlock()
}

// IgnorePatterns returns a copy of compiled ignore patterns for read-only use.
func (s *Settings) IgnorePatterns() []*regexp.Regexp {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.ignorePatterns) == 0 {
		return nil
	}
	out := make([]*regexp.Regexp, len(s.ignorePatterns))
	copy(out, s.ignorePatterns)
	return out
}

// ShouldIgnore returns true if path matches any ignore pattern.
func (s *Settings) ShouldIgnore(path string) bool {
	patterns := s.IgnorePatterns()
	for _, re := range patterns {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}
