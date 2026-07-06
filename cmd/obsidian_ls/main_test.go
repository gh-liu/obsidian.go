package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenLogFileCreatesFileInHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	file, err := openLogFile()
	if err != nil {
		t.Fatalf("openLogFile: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	info, err := os.Stat(filepath.Join(home, ".obsidian_ls.log"))
	if err != nil {
		t.Fatalf("stat log file: %v", err)
	}
	if info.IsDir() {
		t.Fatal("log path is a directory, want file")
	}
}
