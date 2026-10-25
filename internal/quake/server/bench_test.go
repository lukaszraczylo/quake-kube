package server

import "testing"

func BenchmarkConfigMarshal(b *testing.B) {
	cfg := Default()
	for i := 0; i < 50; i++ {
		cfg.Maps = append(cfg.Maps, Map{Name: "q3dm7"})
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := cfg.Marshal(); err != nil {
			b.Fatal(err)
		}
	}
}
