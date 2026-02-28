package lsp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.uber.org/zap"
)

// StartServer starts the LSP server. Reads from stdin, writes to stdout.
func StartServer(logger *slog.Logger) {
	// protocol.ServerDispatcher requires *zap.Logger; use Noop to avoid protocol debug spam
	zapLog := zap.NewNop()
	conn := jsonrpc2.NewConn(jsonrpc2.NewStream(&readWriteCloser{
		reader: os.Stdin,
		writer: os.Stdout,
	}))

	handler, ctx, err := NewHandler(context.Background(), conn, protocol.ServerDispatcher(conn, zapLog), logger)
	if err != nil {
		logger.Error("init handler", "err", err)
		os.Exit(1)
	}

	conn.Go(ctx, protocol.ServerHandler(handler, jsonrpc2.MethodNotFoundHandler))
	<-conn.Done()
}

type readWriteCloser struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func (r *readWriteCloser) Read(b []byte) (int, error)  { return r.reader.Read(b) }
func (r *readWriteCloser) Write(b []byte) (int, error) { return r.writer.Write(b) }
func (r *readWriteCloser) Close() error                { return errors.Join(r.reader.Close(), r.writer.Close()) }
