package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestMediaServerServesOnlyRegisteredFiles(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "Recording 1.mp4")
	if err := os.WriteFile(video, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newMediaServer()
	u, err := m.register(video)
	if err != nil {
		t.Fatal(err)
	}

	// Full file, with the video content type.
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, u, nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "0123456789" || rec.Header().Get("Content-Type") != "video/mp4" {
		t.Fatalf("GET %s = %d %q %q", u, rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}

	// Range request, as the player sends when seeking.
	req := httptest.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("Range", "bytes=4-6")
	rec = httptest.NewRecorder()
	m.ServeHTTP(rec, req)
	body, _ := io.ReadAll(rec.Body)
	if rec.Code != http.StatusPartialContent || string(body) != "456" {
		t.Fatalf("range = %d %q", rec.Code, body)
	}

	// Unknown tokens and other paths are not served.
	for _, p := range []string{"/media/deadbeef/Recording%201.mp4", "/media/", "/index.html"} {
		rec = httptest.NewRecorder()
		m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, rec.Code)
		}
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "a.mp4"), filepath.Join(dir, "b.mp4")
	os.WriteFile(src, []byte("video"), 0o644)
	if err := copyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "video" {
		t.Errorf("copy = %q", b)
	}
	if _, err := os.Stat(dst + ".part"); !os.IsNotExist(err) {
		t.Error("temporary file left behind")
	}
	if same, _ := sameFile(src, src); !same {
		t.Error("sameFile(src, src) = false")
	}
}
