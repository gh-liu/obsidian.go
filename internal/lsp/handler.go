package lsp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gh-liu/obsidian.go/internal/lsp/completion"
	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"log/slog"
)

const (
	diagRefreshDelay = 120 * time.Millisecond
)

// Handler implements the LSP server.
type Handler struct {
	protocol.Server
	conn             jsonrpc2.Conn
	log              *slog.Logger
	settings         *Settings
	index            *index.Index
	positionEncoding string
	openFilesMu      sync.RWMutex
	openFiles        map[string]struct{}
	diagRefreshMu    sync.Mutex
	diagRefreshTimer *time.Timer
}

// NewHandler creates the LSP handler.
func NewHandler(ctx context.Context, conn jsonrpc2.Conn, server protocol.Server, logger *slog.Logger) (*Handler, context.Context, error) {
	return &Handler{
		Server:    server,
		conn:      conn,
		log:       logger,
		settings:  &Settings{},
		openFiles: make(map[string]struct{}),
	}, ctx, nil
}

// Initialize returns server capabilities.
func (h *Handler) Initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	root := resolveVaultRoot(params)
	if root != "" {
		h.index = index.New(root, h.log, func(p string) bool { return h.settings.ShouldIgnore(p) })
	} else {
		h.log.Warn("no vault root; skip indexing")
	}
	h.positionEncoding = extractPositionEncoding(params)
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.TextDocumentSyncKindFull,
			},
			DefinitionProvider:     true,
			ReferencesProvider:     true,
			DocumentSymbolProvider: true,
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{"[", "#", "|"},
			},
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "obsidian-ls",
			Version: "0.1.0",
		},
	}, nil
}

// Initialized runs full index scan.
func (h *Handler) Initialized(ctx context.Context, params *protocol.InitializedParams) error {
	if h.index == nil {
		return nil
	}
	go func() {
		h.fetchSettings(ctx)
		if err := h.index.IndexAll(ctx); err != nil {
			h.log.Error("index all failed", "err", err)
			return
		}
		h.rediagnoseOpenFiles(ctx)
	}()
	return nil
}

func (h *Handler) Shutdown(ctx context.Context) error { return nil }
func (h *Handler) Exit(ctx context.Context) error {
	if h.conn != nil {
		return h.conn.Close()
	}
	return nil
}

func (h *Handler) DidChangeConfiguration(ctx context.Context, params *protocol.DidChangeConfigurationParams) error {
	h.applySettings(params.Settings)
	return nil
}

func (h *Handler) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	if h.index == nil || params == nil {
		return nil
	}
	rel := uriToRelPath(params.TextDocument.URI, h.index.Root())
	if rel == "" {
		return nil
	}
	_ = h.index.SetContent(rel, []byte(params.TextDocument.Text))
	h.trackOpenFile(rel)
	h.index.FlushReparse(rel)
	go diagnoseFile(ctx, h.conn, h.index, rel, h.positionEncoding, []byte(params.TextDocument.Text))
	return nil
}

func (h *Handler) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	if h.index == nil || params == nil || len(params.ContentChanges) == 0 {
		return nil
	}
	rel := uriToRelPath(params.TextDocument.URI, h.index.Root())
	if rel == "" {
		return nil
	}
	newContent := params.ContentChanges[0].Text
	_ = h.index.SetContent(rel, []byte(newContent))
	go diagnoseFile(ctx, h.conn, h.index, rel, h.positionEncoding, []byte(newContent))
	h.scheduleRediagnoseOpenFiles()
	return nil
}

func (h *Handler) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	if h.index == nil || params == nil {
		return nil
	}
	rel := uriToRelPath(params.TextDocument.URI, h.index.Root())
	if rel == "" {
		return nil
	}
	h.index.ClearContent(rel)
	h.untrackOpenFile(rel)
	clearDiagnostics(ctx, h.conn, h.index, rel)
	h.scheduleRediagnoseOpenFiles()
	return nil
}

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
		if strings.HasPrefix(rel, "..") || !strings.HasSuffix(strings.ToLower(filepath.Base(rel)), ".md") {
			continue
		}
		if h.settings.ShouldIgnore(rel) {
			continue
		}
		switch ev.Type {
		case protocol.FileChangeTypeCreated:
			content, _ := os.ReadFile(fullPath)
			_ = h.index.Add(rel, content)
		case protocol.FileChangeTypeChanged:
			if h.index.HasOpenContent(rel) {
				continue
			}
			content, _ := os.ReadFile(fullPath)
			_ = h.index.Update(rel, content)
		case protocol.FileChangeTypeDeleted:
			h.index.Remove(rel)
		}
	}
	h.rediagnoseOpenFiles(ctx)
	return nil
}

func (h *Handler) Definition(ctx context.Context, params *protocol.DefinitionParams) ([]protocol.Location, error) {
	if h.index == nil {
		return nil, nil
	}
	rel := uriToRelPath(params.TextDocument.URI, h.index.Root())
	if rel == "" {
		return nil, nil
	}
	return ResolveDefinition(ctx, h.index, rel, h.positionEncoding, params)
}

func (h *Handler) References(ctx context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	if h.index == nil {
		return nil, nil
	}
	rel := uriToRelPath(params.TextDocument.URI, h.index.Root())
	if rel == "" {
		return nil, nil
	}
	return ResolveReferences(ctx, h.index, rel, h.positionEncoding, params)
}

func (h *Handler) Completion(ctx context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	if h.index == nil {
		return nil, nil
	}
	rel := uriToRelPath(params.TextDocument.URI, h.index.Root())
	if rel == "" {
		return nil, nil
	}
	return completion.ResolveCompletion(ctx, h.index, rel, h.positionEncoding, params, h.settings.GetImagePaths())
}

func (h *Handler) DocumentSymbol(ctx context.Context, params *protocol.DocumentSymbolParams) ([]any, error) {
	if h.index == nil {
		return nil, nil
	}
	rel := uriToRelPath(params.TextDocument.URI, h.index.Root())
	if rel == "" {
		return nil, nil
	}
	symbols, err := ResolveDocumentSymbol(ctx, h.index, rel, h.positionEncoding, params)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(symbols))
	for i := range symbols {
		out[i] = symbols[i]
	}
	return out, nil
}

func (h *Handler) Symbols(ctx context.Context, params *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	if h.index == nil || params == nil {
		return nil, nil
	}
	return ResolveWorkspaceSymbol(ctx, h.index, h.positionEncoding, params)
}

// --- helpers ---

func resolveVaultRoot(params *protocol.InitializeParams) string {
	if len(params.WorkspaceFolders) > 0 {
		return uri.URI(params.WorkspaceFolders[0].URI).Filename()
	}
	if params.RootURI != "" {
		return uri.URI(params.RootURI).Filename()
	}
	return ""
}

func uriToRelPath(docURI protocol.DocumentURI, root string) string {
	fullPath := uri.URI(docURI).Filename()
	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "..") {
		return ""
	}
	return rel
}

func extractPositionEncoding(params *protocol.InitializeParams) string {
	if params == nil {
		return "utf-16"
	}
	if opts, ok := params.InitializationOptions.(map[string]any); ok {
		if s, ok := opts["positionEncoding"].(string); ok && (s == "utf-8" || s == "utf-16") {
			return s
		}
	}
	data, _ := json.Marshal(params.Capabilities)
	var caps struct {
		General struct {
			PositionEncodings []string `json:"positionEncodings"`
		} `json:"general"`
	}
	if err := json.Unmarshal(data, &caps); err != nil {
		return "utf-16"
	}
	if slices.Contains(caps.General.PositionEncodings, "utf-8") {
		return "utf-8"
	}
	return "utf-16"
}

func (h *Handler) fetchSettings(ctx context.Context) {
	if h.conn == nil {
		return
	}
	params := protocol.ConfigurationParams{
		Items: []protocol.ConfigurationItem{{Section: "obsidian"}},
	}
	var result []any
	if _, err := h.conn.Call(ctx, "workspace/configuration", &params, &result); err != nil {
		return
	}
	if len(result) > 0 && result[0] != nil {
		h.applySettings(result[0])
	}
}

func (h *Handler) applySettings(settings any) {
	section := extractObsidianSection(settings)
	if section == nil {
		return
	}
	if v, ok := section["ignores"].([]any); ok {
		h.settings.SetIgnorePatterns(toStrings(v))
	}
	if s, ok := section["templatePath"].(string); ok {
		h.settings.SetTemplatePath(s)
	}
	if v, ok := section["imagePaths"].([]any); ok {
		h.settings.SetImagePaths(toStrings(v))
	}
}

func extractObsidianSection(settings any) map[string]any {
	if settings == nil {
		return nil
	}
	m, ok := settings.(map[string]any)
	if !ok {
		return nil
	}
	if section, ok := m["obsidian"].(map[string]any); ok {
		return section
	}
	return m
}

func toStrings(v []any) []string {
	out := make([]string, 0, len(v))
	for _, x := range v {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func (h *Handler) trackOpenFile(rel string) {
	h.openFilesMu.Lock()
	defer h.openFilesMu.Unlock()
	h.openFiles[rel] = struct{}{}
}

func (h *Handler) untrackOpenFile(rel string) {
	h.openFilesMu.Lock()
	defer h.openFilesMu.Unlock()
	delete(h.openFiles, rel)
}

func (h *Handler) openFilesSnapshot() []string {
	h.openFilesMu.RLock()
	defer h.openFilesMu.RUnlock()
	out := make([]string, 0, len(h.openFiles))
	for p := range h.openFiles {
		out = append(out, p)
	}
	return out
}

func (h *Handler) rediagnoseOpenFiles(ctx context.Context) {
	if h.index == nil {
		return
	}
	for _, rel := range h.openFilesSnapshot() {
		content, err := h.index.GetContent(rel)
		if err != nil {
			continue
		}
		go diagnoseFile(ctx, h.conn, h.index, rel, h.positionEncoding, []byte(content))
	}
}

func (h *Handler) scheduleRediagnoseOpenFiles() {
	if h.index == nil {
		return
	}
	h.diagRefreshMu.Lock()
	if h.diagRefreshTimer != nil {
		h.diagRefreshTimer.Stop()
	}
	var timer *time.Timer
	timer = time.AfterFunc(diagRefreshDelay, func() {
		h.diagRefreshMu.Lock()
		if h.diagRefreshTimer == timer {
			h.diagRefreshTimer = nil
		}
		h.diagRefreshMu.Unlock()
		h.rediagnoseOpenFiles(context.Background())
	})
	h.diagRefreshTimer = timer
	h.diagRefreshMu.Unlock()
}
