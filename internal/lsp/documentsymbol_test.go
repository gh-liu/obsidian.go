package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func TestResolveDocumentSymbol(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	content := `# Title
## Section 1
### Sub 1.1
### Sub 1.2
## Section 2
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	params := &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(path)},
	}

	symbols, err := ResolveDocumentSymbol(context.Background(), idx, "doc.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveDocumentSymbol: %v", err)
	}
	if len(symbols) != 1 {
		t.Fatalf("want 1 root symbol, got %d", len(symbols))
	}
	root := symbols[0]
	if root.Name != "Title" {
		t.Errorf("root name: want Title, got %q", root.Name)
	}
	if len(root.Children) != 2 {
		t.Fatalf("root children: want 2, got %d", len(root.Children))
	}
	if root.Children[0].Name != "Section 1" {
		t.Errorf("child 0: want Section 1, got %q", root.Children[0].Name)
	}
	if root.Children[1].Name != "Section 2" {
		t.Errorf("child 1: want Section 2, got %q", root.Children[1].Name)
	}
	// Section 1 has two subs
	if len(root.Children[0].Children) != 2 {
		t.Fatalf("Section 1 children: want 2, got %d", len(root.Children[0].Children))
	}
	if root.Children[0].Children[0].Name != "Sub 1.1" || root.Children[0].Children[1].Name != "Sub 1.2" {
		t.Errorf("Section 1 subs: got %q, %q", root.Children[0].Children[0].Name, root.Children[0].Children[1].Name)
	}
}

func TestResolveDocumentSymbol_UsesOpenContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(path, []byte("# Disk\n"), 0644); err != nil {
		t.Fatal(err)
	}

	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	if err := idx.SetContent("doc.md", []byte("# Open\n## Child\n")); err != nil {
		t.Fatalf("SetContent: %v", err)
	}

	params := &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(path)},
	}
	symbols, err := ResolveDocumentSymbol(context.Background(), idx, "doc.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveDocumentSymbol: %v", err)
	}
	if len(symbols) != 1 {
		t.Fatalf("want 1 root symbol, got %d", len(symbols))
	}
	if symbols[0].Name != "Open" {
		t.Fatalf("want open-content symbol name Open, got %q", symbols[0].Name)
	}
	if len(symbols[0].Children) != 1 || symbols[0].Children[0].Name != "Child" {
		t.Fatalf("want child heading from open content, got %+v", symbols[0].Children)
	}
}

func TestResolveDocumentSymbol_NoHeadings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(path, []byte("no headings\n"), 0644); err != nil {
		t.Fatal(err)
	}
	idx := index.New(dir, nil, nil)
	if err := idx.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	params := &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(path)},
	}
	symbols, err := ResolveDocumentSymbol(context.Background(), idx, "empty.md", "utf-8", params)
	if err != nil {
		t.Fatalf("ResolveDocumentSymbol: %v", err)
	}
	if symbols != nil {
		t.Errorf("want nil for no headings, got %d symbols", len(symbols))
	}
}
