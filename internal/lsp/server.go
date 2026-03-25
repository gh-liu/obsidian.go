package lsp

import (
	"context"
	"encoding/json"
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

	conn.Go(ctx, serverHandlerWithInlayHint(handler))
	<-conn.Done()
}

func serverHandlerWithInlayHint(handler *Handler) jsonrpc2.Handler {
	base := protocol.ServerHandler(handler, jsonrpc2.MethodNotFoundHandler)
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		switch req.Method() {
		case protocol.MethodInitialize:
			var params protocol.InitializeParams
			if err := json.Unmarshal(req.Params(), &params); err != nil {
				return reply(ctx, nil, err)
			}
			result, err := handler.Initialize(ctx, &params)
			if err != nil {
				return reply(ctx, nil, err)
			}
			raw, err := marshalInitializeResultWithInlayHint(result)
			if err != nil {
				return reply(ctx, nil, err)
			}
			return reply(ctx, raw, nil)
		case methodTextDocumentInlayHint:
			var params InlayHintParams
			if err := json.Unmarshal(req.Params(), &params); err != nil {
				return reply(ctx, nil, err)
			}
			result, err := handler.InlayHint(ctx, &params)
			return reply(ctx, result, err)
		default:
			return base(ctx, reply, req)
		}
	}
}

type readWriteCloser struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func (r *readWriteCloser) Read(b []byte) (int, error)  { return r.reader.Read(b) }
func (r *readWriteCloser) Write(b []byte) (int, error) { return r.writer.Write(b) }
func (r *readWriteCloser) Close() error                { return errors.Join(r.reader.Close(), r.writer.Close()) }
