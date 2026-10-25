package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func newTestRouter(t *testing.T, serverAddr string) http.Handler {
	t.Helper()
	files := fstest.MapFS{
		"index.html": {Data: []byte(`{{define "index"}}ok{{end}}`)},
		"app.js":     {Data: []byte("x")},
	}
	e, err := NewRouter(&Config{ContentServerURL: "http://127.0.0.1:9", ServerAddr: serverAddr, Files: http.FS(files)})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestInfoAndStatusReturn503WhenServerDown(t *testing.T) {
	// Nothing listens on UDP port 9, so the query times out.
	h := newTestRouter(t, "127.0.0.1:9")
	for _, path := range []string{"/info", "/status"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s status = %d, want 503", path, rec.Code)
		}
	}
}

func TestStaticFilesAreCachedButIndexIsNot(t *testing.T) {
	h := newTestRouter(t, "127.0.0.1:9")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app.js", nil))
	if got := rec.Header().Get("Cache-Control"); got != staticCacheMaxAge {
		t.Errorf("static Cache-Control = %q", got)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Errorf("index Cache-Control = %q, want none", got)
	}
}
