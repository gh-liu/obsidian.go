package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gh-liu/obsidian.go/internal/lsp"
)

const logFileName = ".obsidian_ls.log"

func main() {
	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, logFileName)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Default().Error("open log file", "path", logPath, "err", err)
		os.Exit(1)
	}
	defer f.Close()

	logger := slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo}))
	lsp.StartServer(logger)
}
