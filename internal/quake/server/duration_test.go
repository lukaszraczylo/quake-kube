package server

import (
	"testing"
	"time"

	"sigs.k8s.io/yaml"
)

func TestDurationYAML(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"minutes", "d: 15m", 15 * time.Minute, false},
		{"compound", "d: 1h30m", 90 * time.Minute, false},
		{"seconds", "d: 30s", 30 * time.Second, false},
		{"zero", "d: 0s", 0, false},
		{"bad unit", "d: 5 parsecs", 0, true},
		{"bare number", "d: 15", 0, true},
		{"empty string", `d: ""`, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var v struct {
				D Duration `json:"d"`
			}
			err := yaml.Unmarshal([]byte(c.in), &v)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if !c.wantErr && v.D.Duration != c.want {
				t.Fatalf("got %v, want %v", v.D.Duration, c.want)
			}
		})
	}
}

func TestDurationRoundTrip(t *testing.T) {
	out, err := yaml.Marshal(struct {
		D Duration `json:"d"`
	}{Duration{90 * time.Minute}})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "d: 1h30m0s\n" {
		t.Fatalf("got %q", out)
	}
}
