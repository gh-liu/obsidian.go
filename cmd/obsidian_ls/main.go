package main

import (
	"log/slog"
	"os"

	"github.com/gh-liu/obsidian.go/internal/lsp"
)

func main() {
	logger := slog.Default()
	if err := os.MkdirAll(".obsidian_ls.log", 0o755); err == nil {
		// Log to file if possible
	}
	lsp.StartServer(logger)
}
