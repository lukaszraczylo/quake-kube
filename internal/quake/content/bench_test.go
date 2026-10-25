package content

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkManifestCached(b *testing.B) {
	dir := b.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pak0.pk3"), make([]byte, 1<<20), 0o600); err != nil {
		b.Fatal(err)
	}
	st := newStore(dir)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := st.manifest(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkChecksum64MB(b *testing.B) {
	p := filepath.Join(b.TempDir(), "big.pk3")
	if err := os.WriteFile(p, make([]byte, 64<<20), 0o600); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(64 << 20)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := checksum(p); err != nil {
			b.Fatal(err)
		}
	}
}
