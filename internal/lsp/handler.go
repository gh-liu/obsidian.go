package lsp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/gh-liu/obsidian.go/internal/lsp/completion"
	"github.com/gh-liu/obsidian.go/internal/lsp/format"
	"github.com/gh-liu/obsidian.go/internal/lsp/index"
	"github.com/gh-liu/obsidian.go/internal/lsp/position"
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
	openFilesMu      sync.RWMutex
	openFiles        map[string]struct{}
}

// NewHandler creates the LSP handler. Vault root is resolved from Initialize params.
func NewHandler(ctx context.Context, conn jsonrpc2.Conn, server protocol.Server, logger *slog.Logger) (*Handler, context.Context, error) {
	h := &Handler{
		Server:   server,
		conn:     conn,
		log:      logger,
		settings: &Settings{},
		index:    nil, // set after Initialize
		openFiles: make(map[string]struct{}),
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
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.TextDocumentSyncKindFull,
			},
			DefinitionProvider:         true,
			ReferencesProvider:         true,
			DocumentSymbolProvider:     true,
			DocumentFormattingProvider: true,
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{"[", "#", "|"},
			},
			CodeActionProvider:      true,
			WorkspaceSymbolProvider: true,
			ExecuteCommandProvider: &protocol.ExecuteCommandOptions{
				Commands: []string{cmdNew, cmdNewFromTemplate, cmdInsertTemplate, cmdListTemplates, cmdCreateNote},
			},
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
	rel := uriToRelPath(params.TextDocument.URI, h.index.Root())
	if rel == "" {
		return nil, nil
	}
	return ResolveDefinition(ctx, h.index, rel, h.positionEncoding, params)
}

// References returns all links pointing to the target file. Delegates to ResolveReferences.
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

// Completion returns completion items for wiki links ([[file]], [[#heading]], [[path#heading]]).
func (h *Handler) Completion(ctx context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	if h.index == nil {
		return nil, nil
	}
	rel := uriToRelPath(params.TextDocument.URI, h.index.Root())
	if rel == "" {
		return nil, nil
	}
	return completion.ResolveCompletion(ctx, h.index, rel, h.positionEncoding, params)
}

// CodeAction returns quick fixes for diagnostics.
func (h *Handler) CodeAction(ctx context.Context, params *protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	if h.index == nil {
		return nil, nil
	}
	if params == nil {
		return nil, nil
	}
	rel := uriToRelPath(params.TextDocument.URI, h.index.Root())
	if rel == "" {
		return nil, nil
	}
	return ResolveCodeAction(ctx, h.index, rel, h.positionEncoding, params)
}

// Formatting handles textDocument/formatting. Runs format ops in sequence and assembles TextEdits.
func (h *Handler) Formatting(ctx context.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	if h.index == nil || params == nil {
		return nil, nil
	}
	rel := uriToRelPath(params.TextDocument.URI, h.index.Root())
	if rel == "" || !strings.HasSuffix(strings.ToLower(rel), ".md") {
		return nil, nil
	}
	if h.settings.ShouldIgnore(rel) {
		return nil, nil
	}
	content, err := h.index.GetContent(rel)
	if err != nil {
		return nil, nil
	}
	fctx := format.FormatContext{
		Path:  rel,
		Title: strings.TrimSuffix(filepath.Base(rel), ".md"),
		Enc:   position.Encoder{Encoding: h.positionEncoding},
	}
	edits := format.Run(content, fctx, format.DefaultOps)
	if len(edits) == 0 {
		return nil, nil
	}
	return edits, nil
}

// DocumentSymbol returns the document outline (TOC) as a tree of headings.
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
	if len(symbols) == 0 {
		return nil, nil
	}
	out := make([]any, len(symbols))
	for i := range symbols {
		out[i] = symbols[i]
	}
	return out, nil
}

// Symbols returns workspace symbols for notes/headings/tag query.
func (h *Handler) Symbols(ctx context.Context, params *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	if h.index == nil {
		return nil, nil
	}
	if params == nil {
		return nil, nil
	}
	return ResolveWorkspaceSymbol(ctx, h.index, h.positionEncoding, params)
}

// extractPositionEncoding reads position encoding from LSP InitializeParams.
// Supports capabilities.general.positionEncodings and InitializationOptions.positionEncoding.
func extractPositionEncoding(params *protocol.InitializeParams) string {
	if params == nil {
		return "utf-16"
	}
	if opts, ok := params.InitializationOptions.(map[string]any); ok {
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
	if slices.Contains(caps.General.PositionEncodings, "utf-8") {
		return "utf-8"
	}
	return "utf-16"
}

// DidChangeConfiguration is called when workspace settings change.
func (h *Handler) DidChangeConfiguration(ctx context.Context, params *protocol.DidChangeConfigurationParams) error {
	h.applySettings(params.Settings)
	return nil
}

// DidOpen stores document content in index for completion (unsaved state).
func (h *Handler) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	if h.index == nil || params == nil {
		return nil
	}
	rel := uriToRelPath(params.TextDocument.URI, h.index.Root())
	if rel == "" {
		return nil
	}
	if err := h.index.SetContent(rel, []byte(params.TextDocument.Text)); err != nil {
		h.log.Debug("index set content failed", "path", rel, "err", err)
	}
	h.trackOpenFile(rel)
	go diagnoseFile(ctx, h.conn, h.index, rel, h.positionEncoding, []byte(params.TextDocument.Text))
	return nil
}

// DidChange updates document content in index from incremental or full sync.
func (h *Handler) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	if h.index == nil || params == nil {
		return nil
	}
	if len(params.ContentChanges) == 0 {
		return nil
	}
	rel := uriToRelPath(params.TextDocument.URI, h.index.Root())
	if rel == "" {
		return nil
	}
	// Full sync: single change with omitted range (zero value)
	r := params.ContentChanges[0].Range
	if len(params.ContentChanges) == 1 && r.Start.Line == 0 && r.Start.Character == 0 && r.End.Line == 0 && r.End.Character == 0 {
		newContent := params.ContentChanges[0].Text
		if err := h.index.SetContent(rel, []byte(params.ContentChanges[0].Text)); err != nil {
			h.log.Debug("index set content failed", "path", rel, "err", err)
		}
		go diagnoseFile(ctx, h.conn, h.index, rel, h.positionEncoding, []byte(newContent))
		return nil
	}
	// Incremental: apply changes to existing content
	content, err := h.index.GetContent(rel)
	if err != nil || content == "" {
		// Document not in index (e.g. DidOpen not sent); cannot apply incremental
		return nil
	}
	newContent := applyContentChanges(content, params.ContentChanges, position.Encoder{Encoding: h.positionEncoding})
	if err := h.index.SetContent(rel, []byte(newContent)); err != nil {
		h.log.Debug("index set content failed", "path", rel, "err", err)
	}
	go diagnoseFile(ctx, h.conn, h.index, rel, h.positionEncoding, []byte(newContent))
	return nil
}

// DidClose reverts document to disk content in index.
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
	return nil
}

// DidChangeWatchedFiles handles file create/change/delete events from the client.
// Replaces fsnotify for incremental index updates.
// Skips Changed for files that are open (have unsaved content) to avoid overwriting.
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
			if h.index.HasOpenContent(rel) {
				continue
			}
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
	h.rediagnoseOpenFiles(ctx)
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
	var result []any
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

// applySettings extracts settings from LSP workspace configuration.
// Supports: { "ignores": [...], "templatePath": "..." } (workspace/configuration result)
// or { "obsidian": { "ignores": [...], "templatePath": "..." } } (didChangeConfiguration).
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
}

func extractObsidianSection(settings any) map[string]any {
	if settings == nil {
		return nil
	}
	m, ok := settings.(map[string]any)
	if !ok {
		return nil
	}
	// workspace/configuration returns section content directly
	if section, ok := m["obsidian"].(map[string]any); ok {
		return section
	}
	return m
}

func toStrings(v []any) []string {
	var out []string
	for _, p := range v {
		if s, ok := p.(string); ok {
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

// uriToRelPath converts document URI to path relative to root. Returns empty if outside root.
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

// applyContentChanges applies LSP content changes to document. Full sync if any change has no Range.
func applyContentChanges(content string, changes []protocol.TextDocumentContentChangeEvent, enc position.Encoder) string {
	for _, c := range changes {
		if c.Range.Start.Line == 0 && c.Range.Start.Character == 0 && c.Range.End.Line == 0 && c.Range.End.Character == 0 {
			content = c.Text
			continue
		}
		lines := strings.Split(content, "\n")
		startLine := int(c.Range.Start.Line)
		endLine := int(c.Range.End.Line)
		startChar := int(c.Range.Start.Character)
		endChar := int(c.Range.End.Character)
		if startLine < 0 || startLine >= len(lines) {
			continue
		}
		if endLine >= len(lines) {
			endLine = len(lines) - 1
		}
		var before, after string
		before = strings.Join(lines[:startLine], "\n")
		if startLine > 0 {
			before += "\n"
		}
		startByte := enc.CharToByte(lines[startLine], startChar)
		before += lines[startLine][:min(startByte, len(lines[startLine]))]
		if endLine < len(lines) {
			endByte := enc.CharToByte(lines[endLine], endChar)
			after = lines[endLine][min(endByte, len(lines[endLine])):]
			if endLine+1 < len(lines) {
				after += "\n" + strings.Join(lines[endLine+1:], "\n")
			}
		}
		content = before + c.Text + after
	}
	return content
}
