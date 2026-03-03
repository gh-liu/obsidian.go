package format

import (
	"strings"
	"testing"

	"github.com/gh-liu/obsidian.go/internal/lsp/position"
)

func TestFrontmatterOp(t *testing.T) {
	enc := position.Encoder{Encoding: "utf-8"}
	ctx := FormatContext{Title: "Test", Enc: enc}

	t.Run("no frontmatter adds block", func(t *testing.T) {
		content := "# Hello\nbody"
		edit, newContent := FrontmatterOp(content, ctx)
		if newContent == content {
			t.Error("expected content change")
		}
		if edit == nil {
			t.Error("expected edit")
		}
		if !strings.Contains(newContent, "id:") || !strings.Contains(newContent, "title: Test") {
			t.Errorf("expected frontmatter, got %q", newContent)
		}
	})

	t.Run("has all fields no change to body", func(t *testing.T) {
		content := `---
id: 1-AAAA
title: Foo
createdAt: 2025-01-01
updatedAt: 2025-01-01 12:00:00
---
body`
		edit, newContent := FrontmatterOp(content, ctx)
		if !strings.HasSuffix(newContent, "\nbody") {
			t.Errorf("expected body preserved, got %q", newContent)
		}
		if edit != nil && edit.Range.Start.Line != 0 {
			t.Errorf("expected edit in frontmatter range")
		}
	})
}
