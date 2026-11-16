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
