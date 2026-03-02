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
)

// ExecuteCommand handles workspace/executeCommand for obsidian.new and obsidian.newFromTemplate.
func (h *Handler) ExecuteCommand(ctx context.Context, params *protocol.ExecuteCommandParams) (interface{}, error) {
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
	default:
		return nil, fmt.Errorf("unknown command: %s", params.Command)
	}
}

func (h *Handler) executeNew(ctx context.Context, root, templateDir string, args []interface{}) (interface{}, error) {
	targetPath := extractString(args, 0)
	if targetPath == "" {
		targetPath = fmt.Sprintf("Untitled %s.md", time.Now().Format("2006-01-02 15-04-05"))
	}
	targetPath = ensureMdExt(targetPath)
	return h.createNoteFromTemplate(ctx, root, templateDir, "default", targetPath)
}

func (h *Handler) executeNewFromTemplate(ctx context.Context, root, templateDir string, args []interface{}) (interface{}, error) {
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

func (h *Handler) createNoteFromTemplate(ctx context.Context, root, templateDir, templateName, targetPath string) (interface{}, error) {
	if h.settings.ShouldIgnore(targetPath) {
		return nil, fmt.Errorf("path is ignored: %s", targetPath)
	}
	fullPath := filepath.Join(root, targetPath)
	if err := ensureParentDir(fullPath); err != nil {
		return nil, err
	}
	if _, err := os.Stat(fullPath); err == nil {
		return nil, fmt.Errorf("file already exists: %s", targetPath)
	}

	content, err := template.Load(templateDir, templateName)
	if err != nil {
		return nil, fmt.Errorf("load template %s: %w", templateName, err)
	}
	if content == "" && templateName == "default" {
		content = template.BuiltinDefault()
	} else if content == "" {
		return nil, fmt.Errorf("template not found: %s", templateName)
	}

	title := strings.TrimSuffix(filepath.Base(targetPath), ".md")
	args := template.NowArgs(title)
	rendered := template.Execute(content, args)

	if err := os.WriteFile(fullPath, []byte(rendered), 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	if err := h.index.Add(targetPath, []byte(rendered)); err != nil {
		h.log.Debug("index add failed after create", "path", targetPath, "err", err)
	}

	fileURI := string(uri.File(fullPath))
	h.log.Info("created note", "path", targetPath, "template", templateName)
	return map[string]interface{}{"uri": fileURI}, nil
}

func extractString(args []interface{}, i int) string {
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
