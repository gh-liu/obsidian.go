package lsp

import (
	"regexp"
	"sync"
)

// Settings holds obsidian-specific workspace configuration.
type Settings struct {
	mu           sync.RWMutex
	Ignores      []string
	IgnoreREs    []*regexp.Regexp
	TemplatePath string
	ImagePaths   []string
}

// ShouldIgnore returns true if the given path matches any ignore pattern.
func (s *Settings) ShouldIgnore(path string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, re := range s.IgnoreREs {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

// SetIgnorePatterns compiles the given patterns into regexps.
func (s *Settings) SetIgnorePatterns(patterns []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Ignores = patterns
	s.IgnoreREs = nil
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		s.IgnoreREs = append(s.IgnoreREs, re)
	}
}

// SetTemplatePath sets the template directory.
func (s *Settings) SetTemplatePath(p string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TemplatePath = p
}

// GetTemplatePath returns the template directory (default ".templates").
func (s *Settings) GetTemplatePath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.TemplatePath == "" {
		return ".templates"
	}
	return s.TemplatePath
}

func (s *Settings) SetImagePaths(paths []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ImagePaths = paths
}

func (s *Settings) GetImagePaths() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.ImagePaths...)
}
