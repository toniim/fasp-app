package recents

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// MaxStored is how many paths are kept on disk. The tray menu shows a slice.
const MaxStored = 30

// Service tracks recently saved screenshot paths, most recent first.
type Service interface {
	// Add records a saved file path (deduped, moved to the front).
	Add(path string) error
	// List returns existing recent paths, most recent first.
	List() ([]string, error)
	// Folder returns the folder of the most recent existing file.
	Folder() (string, error)
}

type serviceImpl struct {
	mu       sync.Mutex
	filePath string
	paths    []string
}

type fileFormat struct {
	Paths []string `json:"paths"`
}

// New creates a recents service backed by <UserConfigDir>/fasp/recents.json.
func New() Service {
	s := &serviceImpl{}

	dir, err := os.UserConfigDir()
	if err == nil {
		s.filePath = filepath.Join(dir, "fasp", "recents.json")
		s.load()
	}

	return s
}

// Add records path, newest first, without duplicates.
func (s *serviceImpl) Add(path string) error {
	clean := filepath.Clean(path)
	if clean == "" || clean == "." {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Drop any previous occurrence so a re-save moves to the front.
	kept := s.paths[:0]
	for _, p := range s.paths {
		if !samePath(p, clean) {
			kept = append(kept, p)
		}
	}

	s.paths = append([]string{clean}, kept...)
	if len(s.paths) > MaxStored {
		s.paths = s.paths[:MaxStored]
	}

	return s.save()
}

// List returns the recent paths that still exist on disk.
func (s *serviceImpl) List() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := make([]string, 0, len(s.paths))
	changed := false
	for _, p := range s.paths {
		if _, err := os.Stat(p); err != nil {
			changed = true
			continue
		}
		existing = append(existing, p)
	}

	if changed {
		s.paths = existing
		if err := s.save(); err != nil {
			return existing, err
		}
	}

	return append([]string(nil), existing...), nil
}

// Folder returns the directory of the newest existing file.
func (s *serviceImpl) Folder() (string, error) {
	paths, err := s.List()
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", nil
	}
	return filepath.Dir(paths[0]), nil
}

func (s *serviceImpl) load() {
	if s.filePath == "" {
		return
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}

	// Tolerate a UTF-8 BOM (some editors/tools write one when hand-editing).
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	var stored fileFormat
	if err := json.Unmarshal(data, &stored); err != nil {
		return
	}
	s.paths = stored.Paths
}

func (s *serviceImpl) save() error {
	if s.filePath == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(s.filePath), 0700); err != nil {
		return err
	}

	data, err := json.Marshal(fileFormat{Paths: s.paths})
	if err != nil {
		return err
	}

	// 0600: the list reveals what the user has been screenshotting.
	return os.WriteFile(s.filePath, data, 0600)
}

// samePath compares paths the way the host filesystem does.
func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), b)
	}
	return filepath.Clean(a) == b
}
