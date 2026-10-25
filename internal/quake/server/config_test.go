package server

import (
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/yaml"
)

const config = `
fragLimit: 25
timeLimit: 15m
game:
  motd: "Welcome to Critical Stack"
  type: FreeForAll
  forceRespawn: false
  inactivity: 10m
  quadFactor: 3
  weaponRespawn: 3
server:
  hostname: "quakekube"
  maxClients: 12
  password: "changeme"
commands:
- seta g_inactivity 600
- seta sv_timeout 120
maps:
- name: q3dm7
  type: FreeForAll
- name: q3dm17
  type: FreeForAll
- name: q3wctf1
  type: CaptureTheFlag
  captureLimit: 8
- name: q3tourney2
  type: Tournament
- name: q3wctf3
  type: CaptureTheFlag
  captureLimit: 8
- name: ztn3tourney1
  type: Tournament
`

const expectedConfig = `seta fraglimit "25"
seta timelimit "15"
seta bot_minplayers "0"
seta bot_nochat "0"
seta g_forcerespawn "0"
seta g_gametype "0"
seta g_inactivity "600"
seta g_log ""
seta g_motd "Welcome to Critical Stack"
seta g_password ""
seta g_quadfactor "3"
seta g_spSkill "0"
seta g_weaponrespawn "3"
seta fs_basegame ""
seta fs_basepath ""
seta fs_copyfiles "0"
seta fs_debug "0"
seta fs_game ""
seta fs_homepath ""
seta sv_allowDownload "0"
seta sv_hostname "quakekube"
seta sv_maxclients "12"
seta rconpassword "changeme"
set d0 "seta g_gametype 0 ; fraglimit 25 ; timelimit 15 ; map q3dm7 ; set nextmap vstr d1"
set d1 "seta g_gametype 0 ; fraglimit 25 ; timelimit 15 ; map q3dm17 ; set nextmap vstr d2"
set d2 "seta g_gametype 4 ; fraglimit 25 ; timelimit 15 ; capturelimit 8 ; map q3wctf1 ; set nextmap vstr d3"
set d3 "seta g_gametype 1 ; fraglimit 25 ; timelimit 15 ; map q3tourney2 ; set nextmap vstr d4"
set d4 "seta g_gametype 4 ; fraglimit 25 ; timelimit 15 ; capturelimit 8 ; map q3wctf3 ; set nextmap vstr d5"
set d5 "seta g_gametype 1 ; fraglimit 25 ; timelimit 15 ; map ztn3tourney1 ; set nextmap vstr d0"
vstr d0
seta g_inactivity 600
seta sv_timeout 120
`

func TestConfigMarshal(t *testing.T) {
	var cfg *Config
	if err := yaml.Unmarshal([]byte(config), &cfg); err != nil {
		t.Fatal(err)
	}
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != expectedConfig {
		t.Fatalf("config differs\n--- got ---\n%s\n--- want ---\n%s", got, expectedConfig)
	}
}

func TestMapLimitsDoNotLeak(t *testing.T) {
	cfg := Default()
	cfg.FragLimit = 30
	cfg.Maps = Maps{
		{Name: "a", FragLimit: 5, TimeLimit: Duration{Duration: 5 * time.Minute}},
		{Name: "b"},
		{Name: "c", Type: CaptureTheFlag},
	}
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	for _, want := range []string{
		`set d0 "seta g_gametype 0 ; fraglimit 5 ; timelimit 5 ; map a`,
		`set d1 "seta g_gametype 0 ; fraglimit 30 ; timelimit 15 ; map b`,
		`set d2 "seta g_gametype 4 ; fraglimit 30 ; timelimit 15 ; capturelimit 8 ; map c`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestSubMinuteTimeLimitIsNotUnlimited(t *testing.T) {
	if got := minutes(30 * time.Second); got != 1 {
		t.Fatalf("minutes(30s) = %d, want 1", got)
	}
	if got := minutes(0); got != 0 {
		t.Fatalf("minutes(0) = %d, want 0", got)
	}
}

func TestMarshalRejectsBadMapName(t *testing.T) {
	for _, name := range []string{"", "a b", `a"b`, "a;quit", "a\nquit"} {
		cfg := Default()
		cfg.Maps = Maps{{Name: name}}
		if _, err := cfg.Marshal(); err == nil {
			t.Errorf("expected error for map name %q", name)
		}
	}
}

func TestQuakeQuoteStripsBreakers(t *testing.T) {
	if got := quakeQuote("a\"b\nc\\d"); got != `"abc\d"` {
		t.Fatalf("got %s", got)
	}
}

func TestNetworkTuningOptIn(t *testing.T) {
	out, err := Default().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"sv_fps", "sv_maxRate", "sv_dlRate"} {
		if strings.Contains(string(out), key) {
			t.Errorf("%s written without being configured", key)
		}
	}
	var cfg Config
	if uerr := yaml.Unmarshal([]byte("server:\n  fps: 60\n  maxRate: 25000\n  downloadRate: 1000\n"), &cfg); uerr != nil {
		t.Fatal(uerr)
	}
	out, err = cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`seta sv_fps "60"`, `seta sv_maxRate "25000"`, `seta sv_dlRate "1000"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %s", want)
		}
	}
}
