package lsp

import (
	"context"
	"strings"
	"testing"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/protocol"
)

func TestResolveWorkspaceSymbol_TagAndTitleFilter(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "daily.md", `---
tags: [daily, project]
---
# Standup notes
`)
	writeRefFile(t, dir, "random.md", `---
tags: [daily]
---
# Other topic
`)

	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	params := &protocol.WorkspaceSymbolParams{Query: "#daily,project standup"}
	symbols, err := ResolveWorkspaceSymbol(context.Background(), idx, "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveWorkspaceSymbol: %v", err)
	}
	if len(symbols) == 0 {
		t.Fatalf("want symbols, got none")
	}
	found := false
	for _, s := range symbols {
		if strings.EqualFold(s.Name, "Standup notes") || strings.EqualFold(s.Name, "daily") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("want symbol matched by title filter, got %#v", symbols)
	}
}

func TestParseWorkspaceSymbolQuery_MultiTags(t *testing.T) {
	tags, title := parseWorkspaceSymbolQuery("#daily,project,work standup")
	if len(tags) != 3 || tags[0] != "daily" || tags[1] != "project" || tags[2] != "work" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
	if title != "standup" {
		t.Fatalf("unexpected title filter: %q", title)
	}
}
