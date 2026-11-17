package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// VideoInfo opens a video in the in-app player ("open:video" event).
type VideoInfo struct {
	Path      string   `json:"path"`
	URL       string   `json:"url"` // served by mediaServer
	SizeBytes int64    `json:"size_bytes"`
	Warnings  []string `json:"warnings"`
}

// mediaServer lets the WebView play local videos. It serves only files the app
// registered (its own recordings, or one picked from Recents) under an
// unguessable /media/<token>/<name> URL. http.ServeContent answers Range
// requests, so the player can seek without loading the whole file.
type mediaServer struct {
	mu    sync.Mutex
	files map[string]string // token -> path
}

func newMediaServer() *mediaServer {
	return &mediaServer{files: map[string]string{}}
}

func (m *mediaServer) register(path string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	m.mu.Lock()
	m.files[token] = path
	m.mu.Unlock()
	return "/media/" + token + "/" + url.PathEscape(filepath.Base(path)), nil
}

// middleware answers /media/ requests and lets every other request reach the
// embedded frontend assets.
func (m *mediaServer) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/media/") {
			m.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *mediaServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rest, ok := strings.CutPrefix(r.URL.Path, "/media/")
	token, _, _ := strings.Cut(rest, "/")
	m.mu.Lock()
	path, known := m.files[token]
	m.mu.Unlock()
	if !ok || !known {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if strings.EqualFold(filepath.Ext(path), ".mp4") {
		w.Header().Set("Content-Type", "video/mp4")
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

func isVideo(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".mp4")
}

// videoInfo registers path with the media server for the player.
func (a *App) videoInfo(path string, warnings []string) (*VideoInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	u, err := a.media.register(path)
	if err != nil {
		return nil, err
	}
	return &VideoInfo{Path: path, URL: u, SizeBytes: info.Size(), Warnings: warnings}, nil
}

// openVideo shows a video file in the in-app player.
func (a *App) openVideo(path string) {
	v, err := a.videoInfo(path, nil)
	if err != nil {
		println("[error] open video:", err.Error())
		return
	}
	a.emit("open:video", v)
	a.ShowWindow()
}

// CopyFileToClipboard puts a file on the clipboard, ready to paste into a chat
// app, an email or a folder.
func (a *App) CopyFileToClipboard(path string) error {
	return a.clipboardService.CopyFiles([]string{path})
}

// SaveVideoAs copies a video to a place the user picks. It returns the new path,
// or "" when the dialog was cancelled.
func (a *App) SaveVideoAs(src string) (string, error) {
	dialog := a.app.Dialog.SaveFile().
		SetMessage("Save Recording").
		SetFilename(filepath.Base(src)).
		AddFilter("MP4 Video (*.mp4)", "*.mp4")
	if a.window.IsVisible() {
		dialog.AttachToWindow(a.window)
	}
	dst, err := dialog.PromptForSingleSelection()
	if err != nil || dst == "" {
		return "", err
	}
	if !isVideo(dst) {
		dst += ".mp4"
	}
	if same, _ := sameFile(src, dst); same {
		return dst, nil
	}
	if err := copyFile(src, dst); err != nil {
		return "", fmt.Errorf("failed to save %s: %w", dst, err)
	}
	a.recordRecent(dst)
	return dst, nil
}

func sameFile(a, b string) (bool, error) {
	ia, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	ib, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(ia, ib), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err = out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return errors.Join(errors.New("rename"), err)
	}
	return nil
}
