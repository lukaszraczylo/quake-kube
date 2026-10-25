package content

import (
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// CopyAssets against our own content router proves the manifest checksum and
// the download-side CRC check agree.
func TestCopyAssetsRoundTrip(t *testing.T) {
	src := t.TempDir()
	pak := filepath.Join(src, "baseq3", "pak0.pk3")
	if err := os.MkdirAll(filepath.Dir(pak), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pak, []byte("not really a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	e, rerr := NewRouter(&Config{AssetsDir: src})
	if rerr != nil {
		t.Fatal(rerr)
	}
	ts := httptest.NewServer(e)
	defer ts.Close()
	u, _ := url.Parse(ts.URL)

	dst := t.TempDir()
	if cerr := CopyAssets(u, dst); cerr != nil {
		t.Fatal(cerr)
	}
	got, err := os.ReadFile(filepath.Join(dst, "baseq3", "pak0.pk3"))
	if err != nil || string(got) != "not really a zip" {
		t.Fatalf("copied file = %q, err %v", got, err)
	}
}

func TestCopyAssetsRejectsCorruptDownload(t *testing.T) {
	src := t.TempDir()
	pak := filepath.Join(src, "pak0.pk3")
	if err := os.WriteFile(pak, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	e, err := NewRouter(&Config{AssetsDir: src})
	if err != nil {
		t.Fatal(err)
	}
	// Change content after the manifest was built: served bytes no longer match the checksum.
	if err := os.WriteFile(pak, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(e)
	defer ts.Close()
	u, _ := url.Parse(ts.URL)
	dst := t.TempDir()
	if err := CopyAssets(u, dst); err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	if _, err := os.Stat(filepath.Join(dst, "pak0.pk3")); err == nil {
		t.Fatal("corrupt file was written")
	}
}
