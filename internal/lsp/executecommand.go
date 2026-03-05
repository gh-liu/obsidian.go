package lsp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gh-liu/obsidian.go/internal/lsp/template"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

const (
	cmdNew             = "obsidian.new"
	cmdNewFromTemplate = "obsidian.newFromTemplate"
	cmdInsertTemplate  = "obsidian.insertTemplate"
	cmdListTemplates   = "obsidian.listTemplates"
	cmdCreateNote      = "obsidian.createNote"
)

// ExecuteCommand handles workspace/executeCommand for obsidian.new, obsidian.newFromTemplate, obsidian.insertTemplate.
func (h *Handler) ExecuteCommand(ctx context.Context, params *protocol.ExecuteCommandParams) (any, error) {
	if h.index == nil {
		return nil, fmt.Errorf("no vault: open a workspace first")
	}
	root := h.index.Root()
	templateDir := filepath.Join(root, h.settings.TemplatePath())

	switch params.Command {
	case cmdNew:
		return h.executeNew(ctx, root, templateDir, params.Arguments)
	case cmdNewFromTemplate:
		return h.executeNewFromTemplate(ctx, root, templateDir, params.Arguments)
	case cmdInsertTemplate:
		return h.executeInsertTemplate(ctx, templateDir, params.Arguments)
	case cmdListTemplates:
		return h.executeListTemplates(templateDir)
	case cmdCreateNote:
		return h.executeCreateNote(ctx, root, templateDir, params.Arguments)
	default:
		return nil, fmt.Errorf("unknown command: %s", params.Command)
	}
}

func (h *Handler) executeNew(ctx context.Context, root, templateDir string, args []any) (any, error) {
	targetPath := extractString(args, 0)
	if targetPath == "" {
		targetPath = fmt.Sprintf("Untitled %s.md", time.Now().Format("2006-01-02 15-04-05"))
	}
	targetPath = ensureMdExt(targetPath)
	return h.createNoteFromTemplate(ctx, root, templateDir, template.DefaultName, targetPath)
}

func (h *Handler) executeNewFromTemplate(ctx context.Context, root, templateDir string, args []any) (any, error) {
	templateName := extractString(args, 0)
	if templateName == "" {
		return nil, fmt.Errorf("obsidian.newFromTemplate requires template name as first argument")
	}
	targetPath := extractString(args, 1)
	if targetPath == "" {
		targetPath = fmt.Sprintf("Untitled %s.md", time.Now().Format("2006-01-02 15-04-05"))
	}
	targetPath = ensureMdExt(targetPath)
	return h.createNoteFromTemplate(ctx, root, templateDir, templateName, targetPath)
}

func (h *Handler) createNoteFromTemplate(ctx context.Context, root, templateDir, templateName, targetPath string) (any, error) {
	if h.settings.ShouldIgnore(targetPath) {
		return nil, fmt.Errorf("path is ignored: %s", targetPath)
	}

	path := filepath.Join(root, targetPath)
	if err := ensureParentDir(path); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("file already exists: %s", targetPath)
	}

	tmpl, err := template.Load(templateDir, templateName)
	if err != nil {
		return nil, fmt.Errorf("load template %s: %w", templateName, err)
	}
	title := strings.TrimSuffix(filepath.Base(targetPath), ".md")
	content := tmpl.Execute(template.NewVars(title))

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	if err := h.index.Add(targetPath, []byte(content)); err != nil {
		h.log.Debug("index add failed after create", "path", targetPath, "err", err)
	}

	h.log.Info("created note", "path", targetPath, "template", templateName)

	return map[string]any{"uri": uri.File(path)}, nil
}

func (h *Handler) executeInsertTemplate(ctx context.Context, templateDir string, args []any) (any, error) {
	templateName := extractString(args, 0)
	if templateName == "" {
		return nil, fmt.Errorf("obsidian.insertTemplate requires template name as first argument")
	}
	docURI := extractString(args, 1)
	if docURI == "" {
		return nil, fmt.Errorf("obsidian.insertTemplate requires document URI as second argument")
	}
	pos := extractPosition(args, 2)
	if pos == nil {
		return nil, fmt.Errorf("obsidian.insertTemplate requires position {line, character} as third argument")
	}

	tmpl, err := template.Load(templateDir, templateName)
	if err != nil {
		return nil, fmt.Errorf("load template %s: %w", templateName, err)
	}

	// Title from document path (filename without .md)
	// TODO: use title field of Doc
	fullPath := uri.URI(docURI).Filename()
	title := strings.TrimSuffix(filepath.Base(fullPath), ".md")
	if title == "" {
		title = "Untitled"
	}
	content := tmpl.Execute(template.NewVars(title))

	edit := protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentURI][]protocol.TextEdit{
			protocol.DocumentURI(docURI): {{
				Range: protocol.Range{
					Start: *pos,
					End:   *pos,
				},
				NewText: content,
			}},
		},
	}
	applyParams := &protocol.ApplyWorkspaceEditParams{Edit: edit}
	var result protocol.ApplyWorkspaceEditResponse
	if _, err := h.conn.Call(ctx, protocol.MethodWorkspaceApplyEdit, applyParams, &result); err != nil {
		return nil, fmt.Errorf("apply edit: %w", err)
	}
	if !result.Applied && result.FailureReason != "" {
		return nil, fmt.Errorf("edit not applied: %s", result.FailureReason)
	}

	h.log.Info("inserted template", "template", templateName, "uri", docURI)

	return map[string]any{"applied": result.Applied}, nil
}

func (h *Handler) executeListTemplates(templateDir string) (any, error) {
	names, err := template.ListNames(templateDir)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	if names == nil {
		names = []string{}
	}
	return map[string]any{"templates": names}, nil
}

func (h *Handler) executeCreateNote(ctx context.Context, root, templateDir string, args []any) (any, error) {
	targetPath := extractString(args, 0)
	if targetPath == "" {
		return nil, fmt.Errorf("obsidian.createNote requires target name as first argument")
	}
	targetPath = ensureMdExt(targetPath)
	return h.createNoteFromTemplate(ctx, root, templateDir, template.DefaultName, targetPath)
}

func extractPosition(args []any, i int) *protocol.Position {
	if i >= len(args) {
		return nil
	}
	m, ok := args[i].(map[string]any)
	if !ok {
		return nil
	}
	line, _ := toUint32(m["line"])
	char, _ := toUint32(m["character"])
	return &protocol.Position{Line: line, Character: char}
}

func toUint32(v any) (uint32, bool) {
	switch x := v.(type) {
	case float64:
		return uint32(x), true
	case int:
		return uint32(x), true
	default:
		return 0, false
	}
}

func extractString(args []any, i int) string {
	if i >= len(args) {
		return ""
	}
	s, _ := args[i].(string)
	return s
}

func ensureMdExt(path string) string {
	if strings.HasSuffix(strings.ToLower(path), ".md") {
		return path
	}
	return path + ".md"
}

func ensureParentDir(fullPath string) error {
	dir := filepath.Dir(fullPath)
	return os.MkdirAll(dir, 0755)
}
