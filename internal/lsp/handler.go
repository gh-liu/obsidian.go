package lsp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"log/slog"
)

// Handler implements the LSP server.
type Handler struct {
	protocol.Server
	conn             jsonrpc2.Conn
	log              *slog.Logger
	settings         *Settings
	index            *index.Index
	positionEncoding string // "utf-8" or "utf-16", from LSP client
}

// NewHandler creates the LSP handler. Vault root is resolved from Initialize params.
func NewHandler(ctx context.Context, conn jsonrpc2.Conn, server protocol.Server, logger *slog.Logger) (*Handler, context.Context, error) {
	h := &Handler{
		Server:   server,
		conn:     conn,
		log:      logger,
		settings: &Settings{},
		index:    nil, // set after Initialize
	}
	return h, ctx, nil
}

// Initialize returns server capabilities. Extracts vault root and position encoding from params.
func (h *Handler) Initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	root := h.resolveVaultRoot(params)
	if root != "" {
		ignore := func(p string) bool { return h.settings.ShouldIgnore(p) }
		h.index = index.New(root, h.log, ignore)
	} else {
		h.log.Warn("no vault root; skip indexing",
			"workspaceFolders", len(params.WorkspaceFolders),
			"rootURI", params.RootURI,
		)
	}
	h.positionEncoding = extractPositionEncoding(params)
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			DefinitionProvider:     true,
			ReferencesProvider:     true,
			DocumentSymbolProvider: true,
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "obsidian-lsp",
			Version: "0.1.0",
		},
	}, nil
}

// Definition resolves link target at position to the target file. Delegates to ResolveDefinition.
func (h *Handler) Definition(ctx context.Context, params *protocol.DefinitionParams) ([]protocol.Location, error) {
	if h.index == nil {
		return nil, nil
	}
	return ResolveDefinition(ctx, h.index, h.index.Root(), h.positionEncoding, params)
}

// DocumentSymbol returns the document outline (TOC) as a tree of headings.
func (h *Handler) DocumentSymbol(ctx context.Context, params *protocol.DocumentSymbolParams) ([]interface{}, error) {
	if h.index == nil {
		return nil, nil
	}
	symbols, err := ResolveDocumentSymbol(ctx, h.index, h.index.Root(), h.positionEncoding, params)
	if err != nil {
		return nil, err
	}
	if len(symbols) == 0 {
		return nil, nil
	}
	out := make([]interface{}, len(symbols))
	for i := range symbols {
		out[i] = symbols[i]
	}
	return out, nil
}

// extractPositionEncoding reads position encoding from LSP InitializeParams.
// Supports capabilities.general.positionEncodings and InitializationOptions.positionEncoding.
func extractPositionEncoding(params *protocol.InitializeParams) string {
	if params == nil {
		return "utf-16"
	}
	if opts, ok := params.InitializationOptions.(map[string]interface{}); ok {
		if s, ok := opts["positionEncoding"].(string); ok && (s == "utf-8" || s == "utf-16") {
			return s
		}
	}
	data, err := json.Marshal(params.Capabilities)
	if err != nil {
		return "utf-16"
	}
	var caps struct {
		General struct {
			PositionEncodings []string `json:"positionEncodings"`
		} `json:"general"`
	}
	if err := json.Unmarshal(data, &caps); err != nil {
		return "utf-16"
	}
	for _, enc := range caps.General.PositionEncodings {
		if enc == "utf-8" {
			return "utf-8"
		}
	}
	return "utf-16"
}

// DidChangeConfiguration is called when workspace settings change.
func (h *Handler) DidChangeConfiguration(ctx context.Context, params *protocol.DidChangeConfigurationParams) error {
	h.applySettings(params.Settings)
	return nil
}

// DidChangeWatchedFiles handles file create/change/delete events from the client.
// Replaces fsnotify for incremental index updates.
func (h *Handler) DidChangeWatchedFiles(ctx context.Context, params *protocol.DidChangeWatchedFilesParams) error {
	if h.index == nil || params == nil {
		return nil
	}
	root := h.index.Root()
	for _, ev := range params.Changes {
		if ev == nil {
			continue
		}
		fullPath := uri.URI(ev.URI).Filename()
		rel, err := filepath.Rel(root, fullPath)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "..") {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(filepath.Base(rel)), ".md") {
			continue
		}
		if h.settings.ShouldIgnore(rel) {
			continue
		}
		switch ev.Type {
		case protocol.FileChangeTypeCreated:
			content, err := os.ReadFile(fullPath)
			if err != nil {
				h.log.Debug("read file failed", "path", rel, "err", err)
				continue
			}
			if err := h.index.Add(rel, content); err != nil {
				h.log.Debug("index add failed", "path", rel, "err", err)
			} else {
				h.log.Debug("indexed", "path", rel)
			}
		case protocol.FileChangeTypeChanged:
			content, err := os.ReadFile(fullPath)
			if err != nil {
				h.log.Debug("read file failed", "path", rel, "err", err)
				continue
			}
			if err := h.index.Update(rel, content); err != nil {
				h.log.Debug("index update failed", "path", rel, "err", err)
			} else {
				h.log.Debug("updated", "path", rel)
			}
		case protocol.FileChangeTypeDeleted:
			h.index.Remove(rel)
			h.log.Debug("removed", "path", rel)
		}
	}
	return nil
}

// Initialized is called after Initialize. Runs full index scan.
// Incremental updates use workspace/didChangeWatchedFiles (client sends file events).
// IndexAll runs in a goroutine to avoid deadlock: conn.Call must not run in the handler's goroutine
// (same as conn's message loop), otherwise the response can never be received.
func (h *Handler) Initialized(ctx context.Context, params *protocol.InitializedParams) error {
	if h.index == nil {
		return nil
	}
	go func() {
		h.fetchSettings(ctx)
		h.registerFileWatchers(ctx)
		if err := h.index.IndexAll(ctx); err != nil {
			h.log.Error("index all failed", "err", err)
		}
	}()
	return nil
}

func (h *Handler) resolveVaultRoot(params *protocol.InitializeParams) string {
	if len(params.WorkspaceFolders) > 0 {
		return uri.URI(params.WorkspaceFolders[0].URI).Filename()
	}
	if params.RootURI != "" {
		return uri.URI(params.RootURI).Filename()
	}
	return ""
}

// fetchSettings requests workspace configuration from the client.
func (h *Handler) fetchSettings(ctx context.Context) {
	if h.conn == nil {
		return
	}
	params := protocol.ConfigurationParams{
		Items: []protocol.ConfigurationItem{{Section: "obsidian"}},
	}
	var result []interface{}
	if _, err := h.conn.Call(ctx, "workspace/configuration", &params, &result); err != nil {
		h.log.Debug("workspace/configuration failed", "err", err)
		return
	}
	if len(result) > 0 && result[0] != nil {
		h.applySettings(result[0])
	}
}

// registerFileWatchers asks the client to watch **/*.md and send didChangeWatchedFiles.
func (h *Handler) registerFileWatchers(ctx context.Context) {
	if h.conn == nil {
		return
	}
	params := &protocol.RegistrationParams{
		Registrations: []protocol.Registration{
			{
				ID:     "obsidian-watched-files",
				Method: protocol.MethodWorkspaceDidChangeWatchedFiles,
				RegisterOptions: protocol.DidChangeWatchedFilesRegistrationOptions{
					Watchers: []protocol.FileSystemWatcher{
						{GlobPattern: "**/*.md"},
					},
				},
			},
		},
	}
	if _, err := h.conn.Call(ctx, protocol.MethodClientRegisterCapability, params, nil); err != nil {
		h.log.Debug("register file watchers failed", "err", err)
	}
}

// applySettings extracts ignore patterns from LSP workspace settings.
// Supports: { "obsidian": { "ignores": [...] } } (didChangeConfiguration) or { "ignores": [...] } (workspace/configuration result).
func (h *Handler) applySettings(settings interface{}) {
	if settings == nil {
		return
	}
	m, ok := settings.(map[string]interface{})
	if !ok {
		return
	}
	// workspace/configuration returns section content directly: { "ignores": [...] }
	if v, ok := m["ignores"].([]interface{}); ok {
		h.settings.SetIgnorePatterns(toStrings(v))
		return
	}
	// didChangeConfiguration: { "obsidian": { "ignores": [...] } }
	section, ok := m["obsidian"].(map[string]interface{})
	if !ok {
		return
	}
	v, ok := section["ignores"].([]interface{})
	if !ok {
		return
	}
	h.settings.SetIgnorePatterns(toStrings(v))
}

func toStrings(v []interface{}) []string {
	var out []string
	for _, p := range v {
		if s, ok := p.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
