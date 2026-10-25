package server

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type GameType int

const (
	FreeForAll     GameType = 0
	Tournament     GameType = 1
	SinglePlayer   GameType = 2
	TeamDeathmatch GameType = 3
	CaptureTheFlag GameType = 4
)

func (gt GameType) String() string {
	switch gt {
	case FreeForAll:
		return "FreeForAll"
	case Tournament:
		return "Tournament"
	case SinglePlayer:
		return "SinglePlayer"
	case TeamDeathmatch:
		return "TeamDeathmatch"
	case CaptureTheFlag:
		return "CaptureTheFlag"
	default:
		return "Unknown"
	}
}

func (gt *GameType) UnmarshalText(data []byte) error {
	switch string(data) {
	case "FreeForAll", "FFA":
		*gt = FreeForAll
	case "Tournament":
		*gt = Tournament
	case "SinglePlayer":
		*gt = SinglePlayer
	case "TeamDeathmatch":
		*gt = TeamDeathmatch
	case "CaptureTheFlag", "CTF":
		*gt = CaptureTheFlag
	default:
		return fmt.Errorf("unknown GameType: %s", data)
	}
	return nil
}

type Config struct {
	FragLimit int      `name:"fraglimit"`
	TimeLimit Duration `name:"timelimit"`

	BotConfig        `json:"bot"`
	GameConfig       `json:"game"`
	FileServerConfig `json:"fs"`
	ServerConfig     `json:"server"`
	Commands         []string `json:"commands"`

	Maps
}

type BotConfig struct {
	MinPlayers int  `name:"bot_minplayers"`
	NoChat     bool `name:"bot_nochat"`
}

type GameConfig struct {
	ForceRespawn      bool     `name:"g_forcerespawn"`
	GameType          GameType `json:"type" name:"g_gametype"`
	Inactivity        Duration `name:"g_inactivity"`
	Log               string   `name:"g_log"`
	MOTD              string   `name:"g_motd"`
	Password          string   `name:"g_password"`
	QuadFactor        int      `name:"g_quadfactor"`
	SinglePlayerSkill int      `name:"g_spSkill"`
	WeaponRespawn     int      `name:"g_weaponrespawn"`
}

type FileServerConfig struct {
	// allows people to base mods upon mods syntax to follow
	BaseGame string `name:"fs_basegame"`
	// set base path root C:\Program Files\Quake III Arena for files to be
	// downloaded from this path may change for TC's and MOD's
	BasePath string `name:"fs_basepath"`
	// toggle if files can be copied from servers or if client will download
	CopyFiles bool `name:"fs_copyfiles"`
	// possibly enables file server debug mode for download/uploads or
	// something
	Debug bool `name:"fs_debug"`
	// set gamedir set the game folder/dir default is baseq3
	Game string `name:"fs_game"`
	// possibly for TC's and MODS the default is the path to quake3.exe
	HomePath string `name:"fs_homepath"`
}

type ServerConfig struct {
	AllowDownload bool   `name:"sv_allowDownload"`
	DownloadURL   string `name:"sv_dlURL"`
	Hostname      string `name:"sv_hostname"`
	MaxClients    int    `name:"sv_maxclients"`
	Password      string `name:"rconpassword"`
	// Network tuning; 0 leaves the engine default (sv_fps 20, sv_maxRate 0,
	// sv_dlRate 100 kB/s).
	FPS          int `json:"fps" name:"sv_fps,omitempty"`
	MaxRate      int `json:"maxRate" name:"sv_maxRate,omitempty"`
	DownloadRate int `json:"downloadRate" name:"sv_dlRate,omitempty"`
}

const (
	defaultCaptureLimit = 8
	defaultRconPassword = "changeme"
	maxMapNameLen       = 64
)

var (
	durationType = reflect.TypeOf(Duration{})
	mapNameRe    = regexp.MustCompile(`^[A-Za-z0-9_\-]+$`)
)

// Marshal renders the config as a Quake 3 server.cfg.
func (c *Config) Marshal() ([]byte, error) {
	var b bytes.Buffer
	if err := writeStruct(&b, reflect.Indirect(reflect.ValueOf(c))); err != nil {
		return nil, err
	}
	if len(c.Maps) > 0 {
		data, err := c.marshal(c.FragLimit, c.TimeLimit)
		if err != nil {
			return nil, err
		}
		b.Write(data)
	}
	for _, cmd := range c.Commands {
		b.WriteString(cmd)
		b.WriteString("\n")
	}
	return b.Bytes(), nil
}

// writeStruct writes every tagged scalar field; slices are handled by Marshal.
func writeStruct(b *bytes.Buffer, v reflect.Value) error {
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, received %s", v.Kind())
	}
	for i := 0; i < v.Type().NumField(); i++ {
		fv := v.Field(i)
		if fv.Kind() == reflect.Slice {
			continue
		}
		if fv.Kind() == reflect.Struct && fv.Type() != durationType {
			if err := writeStruct(b, fv); err != nil {
				return err
			}
			continue
		}
		tv, ok := v.Type().Field(i).Tag.Lookup("name")
		if !ok {
			continue
		}
		tv, omitEmpty := strings.CutSuffix(tv, ",omitempty")
		if omitEmpty && fv.IsZero() {
			continue
		}
		s, err := toString(v.Type().Field(i).Name, fv)
		if err != nil {
			return err
		}
		if tv == "sv_dlURL" {
			if s != "" {
				fmt.Fprintf(b, "sets %s %s\n", tv, quakeQuote(s))
			}
			continue
		}
		fmt.Fprintf(b, "seta %s %s\n", tv, quakeQuote(s))
	}
	return nil
}

// quakeQuote quotes for the Quake console, which has no escape sequences.
func quakeQuote(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '"' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, s)
	return `"` + s + `"`
}

// minutes rounds up so a sub-minute limit never becomes 0 (unlimited).
func minutes(d time.Duration) int {
	return int((d + time.Minute - 1) / time.Minute)
}

func toString(name string, v reflect.Value) (string, error) {
	switch val := v.Interface().(type) {
	case string:
		return val, nil
	case int:
		return strconv.Itoa(val), nil
	case Duration:
		if name == "TimeLimit" {
			return strconv.Itoa(minutes(val.Duration)), nil
		}
		return strconv.Itoa(int(val.Seconds())), nil
	case bool:
		if val {
			return "1", nil
		}
		return "0", nil
	case GameType:
		return strconv.Itoa(int(val)), nil
	default:
		return "", fmt.Errorf("unsupported config field type %T", val)
	}
}

func Default() *Config {
	return &Config{
		FragLimit: 25,
		TimeLimit: Duration{Duration: 15 * time.Minute},
		Commands:  []string{},
		BotConfig: BotConfig{
			NoChat: true,
		},
		GameConfig: GameConfig{
			Log:               "",
			MOTD:              "Welcome to Critical Stack",
			QuadFactor:        3,
			GameType:          FreeForAll,
			WeaponRespawn:     3,
			Inactivity:        Duration{Duration: 10 * time.Minute},
			SinglePlayerSkill: 2,
			ForceRespawn:      false,
		},
		ServerConfig: ServerConfig{
			MaxClients: 12,
			Hostname:   "quakekube",
			Password:   defaultRconPassword,
		},
		Maps: Maps{
			{Name: "q3dm7", Type: FreeForAll},
			{Name: "q3dm17", Type: FreeForAll},
		},
	}
}

type Map struct {
	Name string   `json:"name"`
	Type GameType `json:"type"`

	CaptureLimit int      `json:"captureLimit"`
	FragLimit    int      `json:"fragLimit"`
	TimeLimit    Duration `json:"timeLimit"`
}

type Maps []Map

// Marshal renders the rotation using no global limits.
func (maps Maps) Marshal() ([]byte, error) {
	return maps.marshal(0, Duration{})
}

// marshal renders the map rotation. Each map sets fraglimit and timelimit
// explicitly, falling back to the globals, so limits never leak between maps.
func (maps Maps) marshal(fragLimit int, timeLimit Duration) ([]byte, error) {
	var b bytes.Buffer
	for i, m := range maps {
		if len(m.Name) > maxMapNameLen || !mapNameRe.MatchString(m.Name) {
			return nil, fmt.Errorf("invalid map name %q", m.Name)
		}
		frag, limit := fragLimit, timeLimit.Duration
		if m.FragLimit != 0 {
			frag = m.FragLimit
		}
		if m.TimeLimit.Duration != 0 {
			limit = m.TimeLimit.Duration
		}
		cmds := []string{
			fmt.Sprintf("g_gametype %d", m.Type),
			fmt.Sprintf("fraglimit %d", frag),
			fmt.Sprintf("timelimit %d", minutes(limit)),
		}
		if m.Type == CaptureTheFlag {
			capture := m.CaptureLimit
			if capture == 0 {
				capture = defaultCaptureLimit
			}
			cmds = append(cmds, fmt.Sprintf("capturelimit %d", capture))
		}
		cmds = append(cmds, fmt.Sprintf("map %s", m.Name))
		nextmap := "d0"
		if i < len(maps)-1 {
			nextmap = fmt.Sprintf("d%d", i+1)
		}
		cmds = append(cmds, fmt.Sprintf("set nextmap vstr %s", nextmap))
		fmt.Fprintf(&b, "set d%d \"seta %s\"\n", i, strings.Join(cmds, " ; "))
	}
	b.WriteString("vstr d0\n")
	return b.Bytes(), nil
}
