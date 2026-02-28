package lsp

import (
	"context"
	"testing"

	"go.lsp.dev/protocol"
)

func TestHandler_DidChangeConfiguration(t *testing.T) {
	h, _, _ := NewHandler(context.Background(), nil, nil, nil)
	// Simulate workspace settings: obsidian.ignores
	params := &protocol.DidChangeConfigurationParams{
		Settings: map[string]interface{}{
			"obsidian": map[string]interface{}{
				"ignores": []interface{}{`^templates/`, `\.git`},
			},
		},
	}
	if err := h.DidChangeConfiguration(context.Background(), params); err != nil {
		t.Fatalf("DidChangeConfiguration: %v", err)
	}
	if !h.settings.ShouldIgnore("templates/foo.md") {
		t.Error("templates/foo.md should be ignored")
	}
	if !h.settings.ShouldIgnore(".git/config") {
		t.Error(".git/config should be ignored")
	}
	if h.settings.ShouldIgnore("a.md") {
		t.Error("a.md should not be ignored")
	}
}

func TestSettings_ShouldIgnore(t *testing.T) {
	s := &Settings{}
	s.SetIgnorePatterns([]string{
		`^templates/`,
		`\.git`,
		`private/.*\.md`,
	})

	tests := []struct {
		path   string
		ignore bool
	}{
		{"a.md", false},
		{"templates/foo.md", true},
		{"templates/sub/x.md", true},
		{".git/config", true},
		{"private/secret.md", true},
		{"public/note.md", false},
	}
	for _, tt := range tests {
		got := s.ShouldIgnore(tt.path)
		if got != tt.ignore {
			t.Errorf("ShouldIgnore(%q): want %v, got %v", tt.path, tt.ignore, got)
		}
	}
}
