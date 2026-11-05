package recents

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestService(t *testing.T) (Service, string) {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("AppData", dir)         // windows
	t.Setenv("XDG_CONFIG_HOME", dir) // linux
	t.Setenv("HOME", dir)            // darwin

	s := New().(*serviceImpl)
	s.filePath = filepath.Join(dir, "recents.json")
	s.paths = nil

	return s, dir
}

func writeFile(t *testing.T, path string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte("png"), 0600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestAddKeepsNewestFirstAndDedupes(t *testing.T) {
	s, dir := newTestService(t)

	first := writeFile(t, filepath.Join(dir, "first.png"))
	second := writeFile(t, filepath.Join(dir, "second.png"))

	if err := s.Add(first); err != nil {
		t.Fatalf("add first: %v", err)
	}
	if err := s.Add(second); err != nil {
		t.Fatalf("add second: %v", err)
	}
	// Re-adding the older file must move it to the front, not duplicate it.
	if err := s.Add(first); err != nil {
		t.Fatalf("re-add first: %v", err)
	}

	got, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(got), got)
	}
	if got[0] != first || got[1] != second {
		t.Fatalf("unexpected order: %v", got)
	}
}

func TestListDropsMissingFiles(t *testing.T) {
	s, dir := newTestService(t)

	kept := writeFile(t, filepath.Join(dir, "kept.png"))
	gone := filepath.Join(dir, "gone.png")
	if err := s.Add(kept); err != nil {
		t.Fatalf("add kept: %v", err)
	}
	if err := s.Add(gone); err != nil {
		t.Fatalf("add gone: %v", err)
	}

	got, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0] != kept {
		t.Fatalf("expected only %s, got %v", kept, got)
	}
}

func TestAddCapsStoredEntries(t *testing.T) {
	s, dir := newTestService(t)

	for i := 0; i < MaxStored+5; i++ {
		p := writeFile(t, filepath.Join(dir, "shot"+string(rune('a'+i%26))+string(rune('0'+i/26))+".png"))
		if err := s.Add(p); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}

	got, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) > MaxStored {
		t.Fatalf("expected at most %d entries, got %d", MaxStored, len(got))
	}
}

func TestFolderReturnsParentOfNewest(t *testing.T) {
	s, dir := newTestService(t)

	nested := filepath.Join(dir, "shots")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := writeFile(t, filepath.Join(nested, "a.png"))
	if err := s.Add(p); err != nil {
		t.Fatalf("add: %v", err)
	}

	folder, err := s.Folder()
	if err != nil {
		t.Fatalf("folder: %v", err)
	}
	if folder != nested {
		t.Fatalf("expected %s, got %s", nested, folder)
	}
}
