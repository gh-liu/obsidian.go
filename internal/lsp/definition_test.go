package lsp

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func TestResolveDefinition_SameNoteChineseHeading(t *testing.T) {
	dir := t.TempDir()
	content := `# 概述

## 其他内容

## 迭代、递归

See [[#迭代、递归]] for details.
`
	writeRefFile(t, dir, "note.md", content)

	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	// Cursor on [[#迭代、递归]] - character 8 is inside the link
	params := &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "note.md"))},
			Position:     protocol.Position{Line: 6, Character: 8},
		},
	}
	locs, err := ResolveDefinition(context.Background(), idx, "note.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveDefinition: %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("want 1 location, got %d", len(locs))
	}
	loc := locs[0]
	// Should jump to "## 迭代、递归" which is on line 4 (0-based)
	if loc.Range.Start.Line != 4 {
		t.Errorf("want line 4 (## 迭代、递归), got line %d", loc.Range.Start.Line)
	}
}

func TestResolveDefinition_DuplicateHeadingSlug(t *testing.T) {
	dir := t.TempDir()
	content := `# Overview

## Intro

## Intro

See [[#intro-1]] for details.
`
	writeRefFile(t, dir, "note.md", content)

	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	params := &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "note.md"))},
			Position:     protocol.Position{Line: 6, Character: 8},
		},
	}
	locs, err := ResolveDefinition(context.Background(), idx, "note.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveDefinition: %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("want 1 location, got %d", len(locs))
	}
	if locs[0].Range.Start.Line != 4 {
		t.Fatalf("want line 4 for second Intro heading, got %d", locs[0].Range.Start.Line)
	}
}
