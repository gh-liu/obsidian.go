package completion

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func writeRefFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveCompletion_HeadingByID_ClosedLink(t *testing.T) {
	// Cursor inside closed link [[1772269373-USPT#工具]] - should still complete.
	dir := t.TempDir()
	writeRefFile(t, dir, "patent.md", `---
id: 1772269373-USPT
---
# 工具
## 子标题`)
	content := "See [[1772269373-USPT#工具]]"
	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	_ = idx.SetContent("note.md", []byte(content))
	// Cursor at position 22: right after #, before 工 (See [[1772269373-USPT#|工具]])
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
		t.Fatalf("expected heading completions when cursor inside [[1772269373-USPT#|工具]], got %v", list)
	}
	labels := make(map[string]bool)
	for _, item := range list.Items {
		labels[item.Label] = true
	}
	for _, want := range []string{"工具", "子标题"} {
		if !labels[want] {
			t.Errorf("missing heading %q when cursor inside closed link, got %v", want, collectLabels(list))
		}
	}
}

func TestResolveCompletion_HeadingByID(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "patent.md", `---
id: 1772269373-USPT
---
# 工具
## 子标题`)
	writeRefFile(t, dir, "note.md", "See [[1772269373-USPT#")
	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	_ = idx.SetContent("note.md", []byte("See [[1772269373-USPT#"))
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
		t.Fatal("expected heading completions for [[1772269373-USPT#")
	}
	labels := make(map[string]bool)
	for _, item := range list.Items {
		labels[item.Label] = true
	}
	for _, want := range []string{"工具", "子标题"} {
		if !labels[want] {
			t.Errorf("missing heading %q, got %v", want, collectLabels(list))
		}
	}
}

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
	if aItem.TextEdit == nil || aItem.TextEdit.NewText != "note-a" {
		t.Errorf("want TextEdit.NewText=note-a, got %+v", aItem.TextEdit)
	}
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

func TestResolveCompletion_CurrentFileBlocks(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "note.md", "line one ^alpha\nline two ^beta\n[[#^")
	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	_ = idx.SetContent("note.md", []byte("line one ^alpha\nline two ^beta\n[[#^"))
	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "note.md"))},
			Position:     protocol.Position{Line: 2, Character: 4},
		},
	}
	list, err := ResolveCompletion(context.Background(), idx, dir, "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	if list == nil {
		t.Fatal("expected non-nil CompletionList")
	}
	labels := make(map[string]bool)
	for _, item := range list.Items {
		labels[item.Label] = true
	}
	for _, want := range []string{"alpha", "beta"} {
		if !labels[want] {
			t.Errorf("missing block completion %q, got %v", want, collectLabels(list))
		}
	}
}

func TestResolveCompletion_TargetFileBlocksByID(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "target.md", "---\nid: target-id\n---\nline one ^alpha\nline two ^beta")
	writeRefFile(t, dir, "note.md", "See [[target-id#^")
	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	_ = idx.SetContent("note.md", []byte("See [[target-id#^"))
	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "note.md"))},
			Position:     protocol.Position{Line: 0, Character: 17},
		},
	}
	list, err := ResolveCompletion(context.Background(), idx, dir, "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	if list == nil || len(list.Items) == 0 {
		t.Fatal("expected block completions for [[target-id#^")
	}
	labels := make(map[string]bool)
	for _, item := range list.Items {
		labels[item.Label] = true
	}
	for _, want := range []string{"alpha", "beta"} {
		if !labels[want] {
			t.Errorf("missing block %q, got %v", want, collectLabels(list))
		}
	}
}

func TestResolveCompletion_ByAlias(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "Artificial Intelligence.md", `---
aliases: [AI, AGI]
---
# Artificial Intelligence`)
	writeRefFile(t, dir, "b.md", `# B`)
	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	_ = idx.SetContent("note.md", []byte("See [[AI"))
	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "note.md"))},
			Position:     protocol.Position{Line: 0, Character: 8},
		},
	}
	list, err := ResolveCompletion(context.Background(), idx, dir, "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	if list == nil {
		t.Fatal("expected non-nil CompletionList")
	}
	var found bool
	for _, item := range list.Items {
		if item.Label == "Artificial Intelligence" {
			found = true
			if item.InsertText != "Artificial Intelligence" {
				t.Errorf("InsertText should be path (no id), got %q", item.InsertText)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected completion for 'Artificial Intelligence' when typing alias 'AI', got labels: %v",
			collectLabels(list))
	}
}

func TestResolveCompletion_FileOrderingByMatchQuality(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "ai-root.md", `# Prefix`)
	writeRefFile(t, dir, "brain.md", `# Contains`)
	writeRefFile(t, dir, "main.md", `# Contains`)
	writeRefFile(t, dir, "zzz.md", `---
aliases: [AIOnly]
---
# Alias only`)
	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	_ = idx.SetContent("note.md", []byte("See [[ai"))
	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "note.md"))},
			Position:     protocol.Position{Line: 0, Character: 8},
		},
	}
	list, err := ResolveCompletion(context.Background(), idx, dir, "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	if list == nil {
		t.Fatal("expected non-nil CompletionList")
	}
	labels := collectLabels(list)
	aiRoot := indexOfLabel(labels, "ai-root")
	brain := indexOfLabel(labels, "brain")
	main := indexOfLabel(labels, "main")
	zzz := indexOfLabel(labels, "zzz")
	if aiRoot < 0 || brain < 0 || main < 0 || zzz < 0 {
		t.Fatalf("missing expected labels in completions: %v", labels)
	}
	if !(aiRoot < brain && brain < zzz) {
		t.Fatalf("want prefix > contains > alias ordering, got %v", labels)
	}
	if !(brain < main) {
		t.Fatalf("want stable alpha order within same score group, got %v", labels)
	}
}

func TestResolveCompletion_HeadingOrderingByMatchQuality(t *testing.T) {
	dir := t.TempDir()
	writeRefFile(t, dir, "note.md", `# T
## Alpha
## Algebra
## Beta Alpha
## Zeta al
[[#al`)
	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	_ = idx.SetContent("note.md", []byte(`# T
## Alpha
## Algebra
## Beta Alpha
## Zeta al
[[#al`))
	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "note.md"))},
			Position:     protocol.Position{Line: 5, Character: 5},
		},
	}
	list, err := ResolveCompletion(context.Background(), idx, dir, "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveCompletion: %v", err)
	}
	if list == nil {
		t.Fatal("expected non-nil CompletionList")
	}
	labels := collectLabels(list)
	algebra := indexOfLabel(labels, "Algebra")
	alpha := indexOfLabel(labels, "Alpha")
	betaAlpha := indexOfLabel(labels, "Beta Alpha")
	zetaAl := indexOfLabel(labels, "Zeta al")
	if algebra < 0 || alpha < 0 || betaAlpha < 0 || zetaAl < 0 {
		t.Fatalf("missing expected labels in completions: %v", labels)
	}
	if !(algebra < alpha && alpha < betaAlpha) {
		t.Fatalf("want heading prefix matches before contains matches, got %v", labels)
	}
	if !(betaAlpha < zetaAl) {
		t.Fatalf("want stable alpha order within contains group, got %v", labels)
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

func collectLabels(list *protocol.CompletionList) []string {
	var out []string
	for _, item := range list.Items {
		out = append(out, item.Label)
	}
	return out
}

func indexOfLabel(labels []string, label string) int {
	for i, v := range labels {
		if v == label {
			return i
		}
	}
	return -1
}
