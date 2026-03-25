package lsp

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func TestExecuteCommand_ShowReferencesDoesNotBlock(t *testing.T) {
	conn := &blockingConn{callBlock: make(chan struct{})}
	h, _, err := NewHandler(context.Background(), conn, nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	h.index = index.New(t.TempDir(), nil, nil)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := h.ExecuteCommand(context.Background(), &protocol.ExecuteCommandParams{
			Command: cmdShowReferences,
			Arguments: []any{
				"file:///tmp/a.md",
				map[string]any{"line": 1, "character": 0},
				[]any{
					map[string]any{
						"uri": "file:///tmp/b.md",
						"range": map[string]any{
							"start": map[string]any{"line": 2, "character": 0},
							"end":   map[string]any{"line": 2, "character": 5},
						},
					},
				},
			},
		})
		if err != nil {
			t.Errorf("ExecuteCommand: %v", err)
		}
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ExecuteCommand blocked on showReferences")
	}

	close(conn.callBlock)
}

func TestExecuteCommand_CreateNoteRediagnosesOpenFiles(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.md")
	writeTestFile(t, sourcePath, "[[1774363699-ZENC]]\n")

	conn := newRecordingConn()
	h, _, err := NewHandler(context.Background(), conn, nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	h.index = index.New(root, nil, nil)
	if err := h.index.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}
	h.trackOpenFile("source.md")

	_, err = h.ExecuteCommand(context.Background(), &protocol.ExecuteCommandParams{
		Command:   cmdCreateNote,
		Arguments: []any{"1774363699-ZENC"},
	})
	if err != nil {
		t.Fatalf("ExecuteCommand: %v", err)
	}

	diag := conn.waitForDiagnostics(t)
	if got := uri.URI(diag.URI).Filename(); got != sourcePath {
		t.Fatalf("diagnostics URI: want %q, got %q", sourcePath, got)
	}
	if len(diag.Diagnostics) != 0 {
		t.Fatalf("expected diagnostics to be cleared, got %d entries", len(diag.Diagnostics))
	}
}

func TestInitialized_RediagnosesOpenFilesAfterInitialIndex(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.md")
	targetPath := filepath.Join(root, "target.md")
	writeTestFile(t, sourcePath, "[[target]]\n")
	writeTestFile(t, targetPath, "# Target\n")

	conn := newRecordingConn()
	h, _, err := NewHandler(context.Background(), conn, nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	h.index = index.New(root, nil, nil)

	if err := h.DidOpen(context.Background(), &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  protocol.DocumentURI(uri.File(sourcePath)),
			Text: "[[target]]\n",
		},
	}); err != nil {
		t.Fatalf("DidOpen: %v", err)
	}

	firstDiag := conn.waitForDiagnostics(t)
	if len(firstDiag.Diagnostics) != 1 {
		t.Fatalf("expected initial unresolved diagnostic, got %d entries", len(firstDiag.Diagnostics))
	}

	if err := h.Initialized(context.Background(), &protocol.InitializedParams{}); err != nil {
		t.Fatalf("Initialized: %v", err)
	}

	finalDiag := conn.waitForDiagnostics(t)
	if got := uri.URI(finalDiag.URI).Filename(); got != sourcePath {
		t.Fatalf("diagnostics URI: want %q, got %q", sourcePath, got)
	}
	if len(finalDiag.Diagnostics) != 0 {
		t.Fatalf("expected diagnostics to be cleared after initial index, got %d entries", len(finalDiag.Diagnostics))
	}
}

func TestDidChange_RediagnosesOtherOpenFilesAfterTargetUpdate(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.md")
	targetPath := filepath.Join(root, "target.md")
	writeTestFile(t, sourcePath, "[[live-id]]\n")
	writeTestFile(t, targetPath, "# Target\n")

	conn := newRecordingConn()
	h, _, err := NewHandler(context.Background(), conn, nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	h.index = index.New(root, nil, nil)
	if err := h.index.IndexAll(context.Background()); err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	if err := h.DidOpen(context.Background(), &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  protocol.DocumentURI(uri.File(sourcePath)),
			Text: "[[live-id]]\n",
		},
	}); err != nil {
		t.Fatalf("DidOpen source: %v", err)
	}
	initialDiag := conn.waitForDiagnosticsForURI(t, sourcePath, func(diag protocol.PublishDiagnosticsParams) bool {
		return len(diag.Diagnostics) == 1
	})
	if len(initialDiag.Diagnostics) != 1 {
		t.Fatalf("expected initial unresolved diagnostic, got %d entries", len(initialDiag.Diagnostics))
	}

	if err := h.DidOpen(context.Background(), &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  protocol.DocumentURI(uri.File(targetPath)),
			Text: "# Target\n",
		},
	}); err != nil {
		t.Fatalf("DidOpen target: %v", err)
	}

	if err := h.DidChange(context.Background(), &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri.File(targetPath))},
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{Text: "---\nid: live-id\n---\n# Target\n"},
		},
	}); err != nil {
		t.Fatalf("DidChange target: %v", err)
	}

	finalDiag := conn.waitForDiagnosticsForURI(t, sourcePath, func(diag protocol.PublishDiagnosticsParams) bool {
		return len(diag.Diagnostics) == 0
	})
	if len(finalDiag.Diagnostics) != 0 {
		t.Fatalf("expected diagnostics to be cleared after target update, got %d entries", len(finalDiag.Diagnostics))
	}
}

type blockingConn struct {
	callBlock chan struct{}
}

func (c *blockingConn) Call(ctx context.Context, method string, params, result interface{}) (jsonrpc2.ID, error) {
	<-c.callBlock
	return jsonrpc2.NewNumberID(1), nil
}

func (c *blockingConn) Notify(ctx context.Context, method string, params interface{}) error {
	return nil
}
func (c *blockingConn) Go(ctx context.Context, handler jsonrpc2.Handler) {}
func (c *blockingConn) Close() error                                     { return nil }
func (c *blockingConn) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (c *blockingConn) Err() error { return nil }

type recordingConn struct {
	notifications chan protocol.PublishDiagnosticsParams
}

func newRecordingConn() *recordingConn {
	return &recordingConn{notifications: make(chan protocol.PublishDiagnosticsParams, 8)}
}

func (c *recordingConn) Call(ctx context.Context, method string, params, result interface{}) (jsonrpc2.ID, error) {
	return jsonrpc2.NewNumberID(1), nil
}

func (c *recordingConn) Notify(ctx context.Context, method string, params interface{}) error {
	if method != protocol.MethodTextDocumentPublishDiagnostics {
		return nil
	}
	p, ok := params.(protocol.PublishDiagnosticsParams)
	if !ok {
		return nil
	}
	c.notifications <- p
	return nil
}

func (c *recordingConn) Go(ctx context.Context, handler jsonrpc2.Handler) {}
func (c *recordingConn) Close() error                                     { return nil }
func (c *recordingConn) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (c *recordingConn) Err() error { return nil }

func (c *recordingConn) waitForDiagnostics(t *testing.T) protocol.PublishDiagnosticsParams {
	t.Helper()
	select {
	case diag := <-c.notifications:
		return diag
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for diagnostics")
		return protocol.PublishDiagnosticsParams{}
	}
}

func (c *recordingConn) waitForDiagnosticsForURI(t *testing.T, path string, match func(protocol.PublishDiagnosticsParams) bool) protocol.PublishDiagnosticsParams {
	t.Helper()
	deadline := time.After(800 * time.Millisecond)
	for {
		select {
		case diag := <-c.notifications:
			if uri.URI(diag.URI).Filename() != path {
				continue
			}
			if match == nil || match(diag) {
				return diag
			}
		case <-deadline:
			t.Fatalf("timed out waiting for diagnostics for %q", path)
			return protocol.PublishDiagnosticsParams{}
		}
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
