package index

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gh-liu/obsidian.go/parse"
)

// Index holds the vault index: path→Doc, id→path.
// Thread-safe.
//
// Incremental updates: Add/Remove/Update allow single-file updates without full re-scan.
// Called from workspace/didChangeWatchedFiles handler. Path must be relative to vault root.
// IgnoreFunc returns true if the path should be ignored. Path is relative to vault root.
type IgnoreFunc func(path string) bool

type Index struct {
	mu            sync.RWMutex
	log           *slog.Logger
	root          string
	ignore        IgnoreFunc
	byPath        map[string]*parse.Doc
	byID          map[string]string // id → path (only when frontmatter has id)
	contentByPath map[string]string // path → raw content for open files (unsaved)
}

// New creates an empty index for the given vault root.
// If log is nil, slog.DiscardHandler is used.
// ignore filters paths during indexing; nil means no filtering.
func New(root string, log *slog.Logger, ignore IgnoreFunc) *Index {
	abs, _ := filepath.Abs(root)
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Index{
		root:          abs,
		log:           log,
		ignore:        ignore,
		byPath:        make(map[string]*parse.Doc),
		byID:          make(map[string]string),
		contentByPath: make(map[string]string),
	}
}

// Root returns the vault root path.
func (x *Index) Root() string {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.root
}

// indexFile reads and parses a single file. rel is relative to root.
func (x *Index) indexFile(rel string) (*parse.Doc, error) {
	content, err := os.ReadFile(filepath.Join(x.root, rel))
	if err != nil {
		return nil, err
	}
	return parse.Parse(content, rel)
}

// IndexAll walks the vault and parses all .md files concurrently.
// Replaces the entire index. Safe to call multiple times.
// Cancelled ctx stops accepting new tasks and returns early.
func (x *Index) IndexAll(ctx context.Context) error {
	paths, err := collectMdFiles(x.root, x.ignore)
	if err != nil {
		return err
	}

	byPath := make(map[string]*parse.Doc)
	byID := make(map[string]string)
	var mu sync.Mutex

	taskCh, wait := NewPool(ctx, 0)
	for _, rel := range paths {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case taskCh <- func() error {
			x.log.Info("index " + rel)
			doc, err := x.indexFile(rel)
			if err != nil {
				return err
			}
			mu.Lock()
			byPath[rel] = doc
			if doc.ID != "" {
				byID[doc.ID] = rel
			}
			mu.Unlock()
			return nil
		}:
		}
	}
	close(taskCh)
	if err := wait(); err != nil {
		return err
	}

	x.mu.Lock()
	x.byPath = byPath
	x.byID = byID
	x.mu.Unlock()

	x.log.Info("index complete", "root", x.root, "files", len(byPath))
	return nil
}

// Add indexes a single file. Called when didChangeWatchedFiles reports Created.
// path is relative to vault root.
func (x *Index) Add(path string, content []byte) error {
	path = filepath.ToSlash(path)
	doc, err := parse.Parse(content, path)
	if err != nil {
		return err
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	x.byPath[path] = doc
	if doc.ID != "" {
		x.byID[doc.ID] = path
	}
	return nil
}

// Remove removes a file from the index. Called when didChangeWatchedFiles reports Deleted.
func (x *Index) Remove(path string) {
	path = filepath.ToSlash(path)
	x.mu.Lock()
	defer x.mu.Unlock()
	if doc, ok := x.byPath[path]; ok && doc.ID != "" {
		delete(x.byID, doc.ID)
	}
	delete(x.byPath, path)
	delete(x.contentByPath, path)
}

// Update re-parses a file. Called when didChangeWatchedFiles reports Changed.
func (x *Index) Update(path string, content []byte) error {
	path = filepath.ToSlash(path)
	doc, err := parse.Parse(content, path)
	if err != nil {
		return err
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if old, ok := x.byPath[path]; ok && old.ID != "" && old.ID != doc.ID {
		delete(x.byID, old.ID)
	}
	x.byPath[path] = doc
	if doc.ID != "" {
		x.byID[doc.ID] = path
	}
	return nil
}

// GetByPath returns the doc for the given relative path, or nil.
func (x *Index) GetByPath(path string) *parse.Doc {
	path = filepath.ToSlash(path)
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.byPath[path]
}

// GetByID returns the path for the given frontmatter id, or empty string.
func (x *Index) GetByID(id string) string {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.byID[id]
}

// ResolveLinkTargetToPath resolves a link target (id or path) to the target file's relative path.
// Returns empty string if not found.
func (x *Index) ResolveLinkTargetToPath(target string) string {
	x.mu.RLock()
	defer x.mu.RUnlock()
	if p := x.byID[target]; p != "" {
		return p
	}
	if _, ok := x.byPath[target]; ok {
		return target
	}
	if !strings.HasSuffix(strings.ToLower(target), ".md") {
		if _, ok := x.byPath[target+".md"]; ok {
			return target + ".md"
		}
	}
	// Obsidian: link by basename when path not found (e.g. [[CS_GO++compiler]] for folder/CS_GO++compiler.md)
	targetBase := strings.TrimSuffix(strings.ToLower(target), ".md")
	for p := range x.byPath {
		base := filepath.Base(p)
		baseNoExt := strings.TrimSuffix(strings.ToLower(base), ".md")
		if baseNoExt == targetBase || base == target {
			return p
		}
	}
	return ""
}

// ListPaths returns all indexed paths (relative to vault root).
func (x *Index) ListPaths() []string {
	x.mu.RLock()
	defer x.mu.RUnlock()
	out := make([]string, 0, len(x.byPath))
	for p := range x.byPath {
		out = append(out, p)
	}
	return out
}

// SetContent stores raw content for an open file and updates the parsed Doc.
// Called on DidOpen/DidChange. path is relative to root.
func (x *Index) SetContent(path string, content []byte) error {
	path = filepath.ToSlash(path)
	doc, err := parse.Parse(content, path)
	if err != nil {
		return err
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	x.contentByPath[path] = string(content)
	if old, ok := x.byPath[path]; ok && old.ID != "" && old.ID != doc.ID {
		delete(x.byID, old.ID)
	}
	x.byPath[path] = doc
	if doc.ID != "" {
		x.byID[doc.ID] = path
	}
	return nil
}

// ClearContent removes open-file content and reverts to disk. Called on DidClose.
// If the file does not exist on disk, removes it from the index.
func (x *Index) ClearContent(path string) {
	path = filepath.ToSlash(path)
	x.mu.Lock()
	defer x.mu.Unlock()
	delete(x.contentByPath, path)
	content, err := os.ReadFile(filepath.Join(x.root, path))
	if err != nil {
		if doc, ok := x.byPath[path]; ok && doc.ID != "" {
			delete(x.byID, doc.ID)
		}
		delete(x.byPath, path)
		return
	}
	doc, err := parse.Parse(content, path)
	if err != nil {
		return
	}
	if old, ok := x.byPath[path]; ok && old.ID != "" && old.ID != doc.ID {
		delete(x.byID, old.ID)
	}
	x.byPath[path] = doc
	if doc.ID != "" {
		x.byID[doc.ID] = path
	}
}

// GetContent returns raw content for the path: from open-file cache if set, else from disk.
func (x *Index) GetContent(path string) (string, error) {
	path = filepath.ToSlash(path)
	x.mu.RLock()
	if c, ok := x.contentByPath[path]; ok {
		x.mu.RUnlock()
		return c, nil
	}
	root := x.root
	x.mu.RUnlock()
	raw, err := os.ReadFile(filepath.Join(root, path))
	return string(raw), err
}

// HasOpenContent returns true if the path has unsaved content (is open).
func (x *Index) HasOpenContent(path string) bool {
	path = filepath.ToSlash(path)
	x.mu.RLock()
	defer x.mu.RUnlock()
	_, ok := x.contentByPath[path]
	return ok
}

// collectMdFiles walks root and returns relative paths of all .md files.
// ignore filters paths; nil means no filtering.
func collectMdFiles(root string, ignore IgnoreFunc) ([]string, error) {
	var paths []string
	err := filepath.Walk(root, func(fullPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, fullPath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if ignore != nil && ignore(rel) {
			return nil
		}
		paths = append(paths, rel)
		return nil
	})
	return paths, err
}
