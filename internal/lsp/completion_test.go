package lsp

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func TestResolveCompletion_FileLinks(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "a.md", `# A`)
	writeRefFile(t, dir, "b.md", `# B`)
	writeRefFile(t, dir, "sub/c.md", `# C`)

	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	_ = idx.SetContent("note.md", []byte("See [["))
	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "note.md"))},
			Position:     protocol.Position{Line: 0, Character: 6},
		},
	}
	list, err := ResolveCompletion(context.Background(), idx, dir, "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	if list == nil {
		t.Fatal("expected non-nil CompletionList")
	}
	if len(list.Items) < 3 {
		t.Errorf("want at least 3 file completions (a, b, sub/c), got %d", len(list.Items))
	}
	labels := make(map[string]bool)
	for _, item := range list.Items {
		labels[item.Label] = true
	}
	for _, want := range []string{"a", "b", "sub/c"} {
		if !labels[want] {
			t.Errorf("missing completion label %q", want)
		}
	}
}

func TestResolveCompletion_HeadingLinks(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "note.md", `# Title
## Section 1
## Section 2
[[#`)
	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	_ = idx.SetContent("note.md", []byte(`# Title
## Section 1
## Section 2
[[#`))
	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "note.md"))},
			Position:     protocol.Position{Line: 3, Character: 3},
		},
	}
	list, err := ResolveCompletion(context.Background(), idx, dir, "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	if list == nil {
		t.Fatal("expected non-nil CompletionList")
	}
	if len(list.Items) < 3 {
		t.Errorf("want at least 3 heading completions, got %d", len(list.Items))
	}
	labels := make(map[string]bool)
	for _, item := range list.Items {
		labels[item.Label] = true
	}
	for _, want := range []string{"Title", "Section 1", "Section 2"} {
		if !labels[want] {
			t.Errorf("missing heading completion %q", want)
		}
	}
}

func TestResolveCompletion_HeadingByBasename(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "lang/CS_GO++compiler.md", `# Compiler
## Frontend
## Backend`)
	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	_ = idx.SetContent("note.md", []byte("See [[CS_GO++compiler#"))
	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "note.md"))},
			Position:     protocol.Position{Line: 0, Character: 22},
		},
	}
	list, err := ResolveCompletion(context.Background(), idx, dir, "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	if list == nil || len(list.Items) == 0 {
		t.Fatal("expected heading completions for [[CS_GO++compiler#")
	}
	labels := make(map[string]bool)
	for _, item := range list.Items {
		labels[item.Label] = true
	}
	for _, want := range []string{"Compiler", "Frontend", "Backend"} {
		if !labels[want] {
			t.Errorf("missing heading %q", want)
		}
	}
}

func TestResolveCompletion_InsertIdWhenAvailable(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "a.md", `---
id: note-a
---
# A`)
	writeRefFile(t, dir, "b.md", `# B`)
	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	_ = idx.SetContent("note.md", []byte("[["))
	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "note.md"))},
			Position:     protocol.Position{Line: 0, Character: 2},
		},
	}
	list, err := ResolveCompletion(context.Background(), idx, dir, "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	if list == nil {
		t.Fatal("expected non-nil CompletionList")
	}
	var aItem *protocol.CompletionItem
	for i := range list.Items {
		if list.Items[i].Label == "a" {
			aItem = &list.Items[i]
			break
		}
	}
	if aItem == nil {
		t.Fatal("expected completion for a.md")
	}
	if aItem.InsertText != "note-a" {
		t.Errorf("want InsertText=note-a (id), got %q", aItem.InsertText)
	}
	// b.md has no id, should insert path
	var bItem *protocol.CompletionItem
	for i := range list.Items {
		if list.Items[i].Label == "b" {
			bItem = &list.Items[i]
			break
		}
	}
	if bItem != nil && bItem.InsertText != "b" {
		t.Errorf("want InsertText=b (path), got %q", bItem.InsertText)
	}
}

func TestResolveCompletion_HeadingTriggerRace(t *testing.T) {
	// When user types "#", Completion may arrive before DidChange. Doc has [[id without #.
	dir := t.TempDir()
	writeRefFile(t, dir, "1770130136-TOWG.md", `# Title
## Section`)
	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	_ = idx.SetContent("note.md", []byte("[[1770130136-TOWG")) // no # yet
	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "note.md"))},
			Position:     protocol.Position{Line: 0, Character: 18},
		},
		Context: &protocol.CompletionContext{TriggerCharacter: "#", TriggerKind: protocol.CompletionTriggerKindTriggerCharacter},
	}
	list, err := ResolveCompletion(context.Background(), idx, dir, "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	if list == nil || len(list.Items) == 0 {
		t.Fatal("expected heading completions when triggered by # (race fix)")
	}
	labels := make(map[string]bool)
	for _, item := range list.Items {
		labels[item.Label] = true
	}
	for _, want := range []string{"Title", "Section"} {
		if !labels[want] {
			t.Errorf("missing heading %q", want)
		}
	}
}

func TestResolveCompletion_OutsideWikiLink(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "a.md", `# A`)
	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	_ = idx.SetContent("note.md", []byte("plain text"))
	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "note.md"))},
			Position:     protocol.Position{Line: 0, Character: 5},
		},
	}
	list, err := ResolveCompletion(context.Background(), idx, dir, "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	if list != nil && len(list.Items) > 0 {
		t.Errorf("expected no completions outside wiki link, got %d", len(list.Items))
	}
}
