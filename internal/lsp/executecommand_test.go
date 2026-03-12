package lsp

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
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
