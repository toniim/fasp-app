package upload

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/heytonyne/fasp/internal/model"
	"github.com/heytonyne/fasp/internal/service/settings"
)

// fakeSettings serves a fixed settings value; the other methods are not used
// by the upload client.
type fakeSettings struct {
	settings.Service
	cfg model.Settings
}

func (f fakeSettings) GetAll() (*model.Settings, error) {
	c := f.cfg
	return &c, nil
}

// initServer records the body of POST /api/upload/init.
func initServer(t *testing.T) (*httptest.Server, *map[string]any) {
	t.Helper()
	got := map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/upload/init" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer fsk_live_test" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("init body is not JSON: %v", err)
		}
		_, _ = w.Write([]byte(`{"file_id":"f1","upload_url":"https://r2.example/put"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

// The init request carries is_public every time, including false: an explicit
// choice overrides the default, and an empty choice follows the setting.
func TestInitSendsIsPublic(t *testing.T) {
	cases := []struct {
		name          string
		defaultPublic bool
		visibility    string
		want          bool
	}{
		{"default public", true, VisibilityDefault, true},
		{"default private", false, VisibilityDefault, false},
		{"explicit private beats a public default", true, VisibilityPrivate, false},
		{"explicit public beats a private default", false, VisibilityPublic, true},
	}
	for _, c := range cases {
		srv, got := initServer(t)
		svc := New(fakeSettings{cfg: model.Settings{ServerURL: srv.URL, APIKey: "fsk_live_test", UploadPublic: c.defaultPublic}})

		resp, err := svc.Init("shot.png", 123, "image/png", c.visibility)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if resp.FileID != "f1" {
			t.Errorf("%s: file id = %q", c.name, resp.FileID)
		}
		v, present := (*got)["is_public"]
		if !present {
			t.Errorf("%s: is_public missing from the init body %v", c.name, *got)
			continue
		}
		if v != c.want {
			t.Errorf("%s: is_public = %v, want %v", c.name, v, c.want)
		}
		if (*got)["filename"] != "shot.png" || (*got)["mime_type"] != "image/png" || (*got)["size"] != float64(123) {
			t.Errorf("%s: init body changed shape: %v", c.name, *got)
		}
	}
}

func TestInitRejectsUnknownVisibility(t *testing.T) {
	srv, _ := initServer(t)
	svc := New(fakeSettings{cfg: model.Settings{ServerURL: srv.URL, APIKey: "fsk_live_test", UploadPublic: true}})
	if _, err := svc.Init("shot.png", 1, "image/png", "friends"); err == nil {
		t.Error("an unknown visibility must be an error, not a guess")
	}
}
