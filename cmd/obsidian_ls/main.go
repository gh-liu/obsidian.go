package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gh-liu/obsidian.go/internal/lsp"
)

func main() {
	logger := slog.Default()
	if file, err := openLogFile(); err == nil {
		defer file.Close()
		logger = slog.New(slog.NewTextHandler(file, nil))
	}
	lsp.StartServer(logger)
}

func openLogFile() (*os.File, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(home, ".obsidian_ls.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}
