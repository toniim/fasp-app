package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/heytonyne/fasp/internal/model"
)

func newTestService(t *testing.T, existing string) *serviceImpl {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if existing != "" {
		if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := &serviceImpl{settings: model.DefaultSettings(), filePath: path}
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	return s
}

// A settings file written before freeze-zone and recording existed keeps its
// own hotkeys and gains the new defaults.
func TestLoadKeepsNewHotkeyDefaults(t *testing.T) {
	s := newTestService(t, `{"hotkeys":{"capture_fullscreen":"F9"},"default_format":"jpeg"}`)
	hk := s.settings.Hotkeys
	if hk["capture_fullscreen"] != "F9" {
		t.Errorf("user hotkey lost: %q", hk["capture_fullscreen"])
	}
	def := model.DefaultSettings().Hotkeys
	for _, k := range []string{"capture_zone", "record"} {
		if hk[k] == "" || hk[k] != def[k] {
			t.Errorf("%s = %q, want default %q", k, hk[k], def[k])
		}
	}
	if !s.settings.RecordSystemAudio || s.settings.RecordFPS != 30 {
		t.Errorf("recording defaults missing: %+v", s.settings)
	}
}

func TestSetRecordingSettings(t *testing.T) {
	s := newTestService(t, "")
	for key, v := range map[string]interface{}{
		"record_system_audio":  false,
		"record_microphone":    true,
		"record_mic_device":    "{0.0.1.00000000}.{abc}",
		"record_fps":           float64(60), // numbers arrive from JSON as float64
		"hotkeys.capture_zone": "Ctrl+Shift+Z",
	} {
		if err := s.Set(key, v); err != nil {
			t.Fatalf("Set(%s): %v", key, err)
		}
	}
	if err := s.Set("record_fps", float64(25)); err == nil {
		t.Error("record_fps 25 should be rejected")
	}

	reloaded := newTestService(t, "")
	reloaded.filePath = s.filePath
	if err := reloaded.Load(); err != nil {
		t.Fatal(err)
	}
	got := reloaded.settings
	if got.RecordSystemAudio || !got.RecordMicrophone || got.RecordMicDevice == "" || got.RecordFPS != 60 ||
		got.Hotkeys["capture_zone"] != "Ctrl+Shift+Z" {
		t.Errorf("settings not persisted: %+v", got)
	}
}

// The editor's tool settings come back from the file as the frontend wrote
// them, nested values included; a file without them still loads.
func TestEditorPrefsRoundTrip(t *testing.T) {
	old := newTestService(t, `{"default_format":"jpeg"}`)
	if old.settings.EditorPrefs != nil {
		t.Errorf("old settings file should have no editor prefs, got %v", old.settings.EditorPrefs)
	}

	s := newTestService(t, "")
	prefs := map[string]interface{}{
		"blurMode":        "pixelate",
		"rectangleRadius": float64(12),
		"backdrop":        map[string]interface{}{"enabled": true, "ratio": "16:9", "padding": float64(8)},
	}
	if err := s.Set("editor_prefs", prefs); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("editor_prefs", "not an object"); err == nil {
		t.Error("editor_prefs must be an object")
	}

	reloaded := newTestService(t, "")
	reloaded.filePath = s.filePath
	if err := reloaded.Load(); err != nil {
		t.Fatal(err)
	}
	got := reloaded.settings.EditorPrefs
	backdrop, _ := got["backdrop"].(map[string]interface{})
	if got["blurMode"] != "pixelate" || got["rectangleRadius"] != float64(12) || backdrop["ratio"] != "16:9" || backdrop["enabled"] != true {
		t.Errorf("editor prefs not persisted: %v", got)
	}
}
