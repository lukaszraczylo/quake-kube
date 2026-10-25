package content

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSafeJoin(t *testing.T) {
	cases := []struct {
		rel string
		ok  bool
	}{
		{"baseq3/pak0.pk3", true},
		{".", true},
		{"../etc/passwd", false},
		{"a/../../b", false},
		{"/etc/passwd", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run(c.rel, func(t *testing.T) {
			_, err := safeJoin("/srv/assets", c.rel)
			if (err == nil) != c.ok {
				t.Fatalf("safeJoin(%q) err = %v, want ok=%v", c.rel, err, c.ok)
			}
		})
	}
}

func upload(t *testing.T, h http.Handler, name, filename string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("name", name); err != nil {
		t.Fatal(err)
	}
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/maps", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestUploadRejectsTraversalAndRefreshesManifest(t *testing.T) {
	root := t.TempDir()
	assets := filepath.Join(root, "assets")
	if err := os.MkdirAll(assets, 0o750); err != nil {
		t.Fatal(err)
	}
	e, err := NewRouter(&Config{AssetsDir: assets})
	if err != nil {
		t.Fatal(err)
	}

	if rec := upload(t, e, "../outside", "evil.pk3", []byte("x")); rec.Code != http.StatusBadRequest {
		t.Fatalf("traversal upload status = %d, want 400", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(root, "outside")); err == nil {
		t.Fatal("upload escaped the assets dir")
	}

	if rec := upload(t, e, "baseq3", "extra.pk3", []byte("data")); rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body %s", rec.Code, rec.Body)
	}
	req := httptest.NewRequest(http.MethodGet, "/assets/manifest.json", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var files []File
	if err := json.Unmarshal(rec.Body.Bytes(), &files); err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name != "baseq3/extra.pk3" {
		t.Fatalf("manifest after upload = %+v", files)
	}
}

func TestAssetRouteRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	assets := filepath.Join(root, "assets")
	if err := os.MkdirAll(assets, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret"), []byte("s"), 0o600); err != nil {
		t.Fatal(err)
	}
	e, err := NewRouter(&Config{AssetsDir: assets})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/assets/1-..%2Fsecret", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("traversal served: %s", rec.Body)
	}
}

func TestUploadTokenAndEmptyPack(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "empty.pk3"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	e, err := NewRouter(&Config{AssetsDir: dir, UploadToken: "s3cret"})
	if err != nil {
		t.Fatal(err)
	}
	// Live regression: a 0-byte pk3 used to turn GET /maps into a 500.
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/maps", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/maps status = %d", rec.Code)
	}
	if rec := upload(t, e, "baseq3", "x.pk3", []byte("d")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated upload status = %d, want 401", rec.Code)
	}
	for _, c := range []struct {
		token, header string
		ok            bool
	}{{"", "", true}, {"t", "Bearer t", true}, {"t", "Bearer x", false}, {"t", "", false}, {"t", "t", false}} {
		if got := uploadAllowed(c.token, c.header); got != c.ok {
			t.Errorf("uploadAllowed(%q,%q)=%v", c.token, c.header, got)
		}
	}
}

func BenchmarkContentParallel(b *testing.B) {
	dir := b.TempDir()
	e, err := NewRouter(&Config{AssetsDir: dir})
	if err != nil {
		b.Fatal(err)
	}
	for _, path := range []string{"/assets/manifest.json", "/maps"} {
		b.Run(path, func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					rec := httptest.NewRecorder()
					e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
					if rec.Code != http.StatusOK {
						b.Fatalf("status %d", rec.Code)
					}
				}
			})
		})
	}
}
