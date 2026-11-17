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

// The last zone survives a restart, can be cleared, and a settings file from
// before it existed loads with no zone.
func TestLastZoneRoundTrip(t *testing.T) {
	if old := newTestService(t, `{"default_format":"jpeg"}`); old.settings.LastZone != nil {
		t.Fatalf("old settings file should have no last zone, got %+v", old.settings.LastZone)
	}

	s := newTestService(t, "")
	zone := &model.ZoneMemory{
		DisplayX: -1440, DisplayY: 0, DisplayWidth: 1440, DisplayHeight: 900,
		X: 10.5, Y: 20, Width: 300.5, Height: 200,
	}
	if err := s.Set("last_zone", zone); err != nil {
		t.Fatal(err)
	}
	zone.X = 999 // the stored zone is a copy
	if err := s.Set("last_zone", "not a zone"); err == nil {
		t.Error("last_zone must be a zone")
	}

	reloaded := newTestService(t, "")
	reloaded.filePath = s.filePath
	if err := reloaded.Load(); err != nil {
		t.Fatal(err)
	}
	got := reloaded.settings.LastZone
	if got == nil || got.DisplayX != -1440 || got.X != 10.5 || got.Width != 300.5 || got.Height != 200 {
		t.Fatalf("last zone not persisted: %+v", got)
	}

	if err := s.Set("last_zone", nil); err != nil {
		t.Fatal(err)
	}
	if s.settings.LastZone != nil {
		t.Errorf("last zone not cleared: %+v", s.settings.LastZone)
	}
}

// Uploads are public unless the user turns it off: that is what the app sent
// before the setting existed, so an old settings file keeps it.
func TestUploadPublicDefault(t *testing.T) {
	old := newTestService(t, `{"default_format":"jpeg"}`)
	if !old.settings.UploadPublic {
		t.Error("an old settings file must keep uploads public")
	}
	if err := old.Set("upload_public", false); err != nil {
		t.Fatal(err)
	}
	reloaded := newTestService(t, "")
	reloaded.filePath = old.filePath
	if err := reloaded.Load(); err != nil {
		t.Fatal(err)
	}
	if reloaded.settings.UploadPublic {
		t.Error("upload_public=false was not persisted")
	}
	if err := old.Set("upload_public", "yes"); err == nil {
		t.Error("upload_public must be a bool")
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
