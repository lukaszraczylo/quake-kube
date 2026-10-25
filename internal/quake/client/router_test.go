package client

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
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

// fakeInfoServer answers the first getinfo and then stays silent, like
// ioq3ded rate-limiting a repeat query from the same address.
func fakeInfoServer(t *testing.T) string {
	t.Helper()
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	go func() {
		buf := make([]byte, 2048)
		answered := false
		for {
			_, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			if answered {
				continue
			}
			answered = true
			_, _ = pc.WriteTo([]byte("\xff\xff\xff\xffinfoResponse\n\\mapname\\q3dm7\\clients\\2"), addr)
		}
	}()
	return pc.LocalAddr().String()
}

func resetInfoCache() {
	infoCache.Lock()
	infoCache.info = nil
	infoCache.lastFetch = time.Time{}
	infoCache.Unlock()
}

func TestInfoServesStaleAnswerWhenRefreshFails(t *testing.T) {
	resetInfoCache()
	t.Cleanup(resetInfoCache)
	h := newTestRouter(t, fakeInfoServer(t))

	get := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/info", nil))
		return rec
	}
	if rec := get(); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "q3dm7") {
		t.Fatalf("first /info = %d %s", rec.Code, rec.Body.String())
	}

	// Expire the cache so the next call must refresh; the server stays silent.
	infoCache.Lock()
	infoCache.lastFetch = time.Now().Add(-time.Hour)
	infoCache.Unlock()
	if rec := get(); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "q3dm7") {
		t.Fatalf("stale /info = %d %s, want the cached 200", rec.Code, rec.Body.String())
	}
}

func TestInfoReturns503WithNoCacheAndNoServer(t *testing.T) {
	resetInfoCache()
	t.Cleanup(resetInfoCache)
	h := newTestRouter(t, "127.0.0.1:9")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/info", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
