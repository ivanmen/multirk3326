package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type MulticastConfig struct {
	Group       string `json:"group"`
	Port        int    `json:"port"`
	IntervalSec int    `json:"intervalSec"`
	Iface       string `json:"iface"`
}

type Config struct {
	Name string `json:"name"`
	// DataDir holds runtime.json (device id, paired clients)
	// and the config file itself. Tiny, written only on pairing/boot.
	DataDir string `json:"dataDir"`
	// RomsRoot is where ArkOS mounts the SD card roms, usually /roms.
	RomsRoot string `json:"romsRoot"`
	// StateDirs / SaveDirs are RetroArch savestate_directory and
	// savefile_directory. Autodetected from retroarch.cfg when empty.
	StateDirs    []string `json:"stateDirs"`
	SaveDirs     []string `json:"saveDirs"`
	RetroArchCfg string   `json:"retroarchCfg"`
	// Protocol is "tcp" (default, compact binary frames) or "http" (debug).
	Protocol string `json:"protocol"`
	Listen   string `json:"listen"`
	// HTTPListen is optional; empty means HTTP is disabled. Useful for curl.
	HTTPListen string `json:"httpListen"`
	// SpoolDir receives uploads before they are atomically moved to the SD.
	// Put it on tmpfs (/dev/shm) so a failed transfer costs zero SD writes.
	SpoolDir        string `json:"spoolDir"`
	Auth            bool   `json:"auth"`
	ScanIntervalSec int    `json:"scanIntervalSec"`
	StateRefreshSec int    `json:"stateRefreshSec"`
	MaxUploadMB     int    `json:"maxUploadMB"`
	Backups         bool   `json:"backups"`
	// AllowControl enables volume/mute/brightness over the API.
	AllowControl bool              `json:"allowControl"`
	AudioControl string            `json:"audioControl"`
	Backlight    string            `json:"backlight"`
	Multicast    MulticastConfig   `json:"multicast"`
	ExcludeDirs  []string          `json:"excludeDirs"`
	SystemNames  map[string]string `json:"systemNames"`

	// Path of the loaded config file (not serialized).
	Path string `json:"-"`
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "multirk"
	}
	return h
}

func defaultDataDir() string {
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "multirk")
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), "multirk")
	}
	return "/home/ark/.config/multirk"
}

// ArkOS / EmulationStation folder names.
var defaultSystemNames = map[string]string{
	"amiga": "Amiga", "amstradcpc": "Amstrad CPC", "arcade": "Arcade",
	"atari2600": "Atari 2600", "atari5200": "Atari 5200", "atari7800": "Atari 7800",
	"atari800": "Atari 800", "atarilynx": "Atari Lynx", "atarijaguar": "Atari Jaguar",
	"c64": "Commodore 64", "coleco": "ColecoVision", "colecovision": "ColecoVision",
	"dreamcast": "Dreamcast", "fba": "FBA", "fbneo": "FBNeo", "gamegear": "Game Gear",
	"gb": "Game Boy", "gba": "Game Boy Advance", "gbc": "Game Boy Color",
	"genesis": "Genesis", "intellivision": "Intellivision", "j2me": "J2ME",
	"mame": "MAME", "mastersystem": "Master System", "megadrive": "Mega Drive",
	"msx": "MSX", "n64": "Nintendo 64", "naomi": "Naomi", "neogeo": "Neo Geo",
	"neogeocd": "Neo Geo CD", "nes": "NES", "ngp": "Neo Geo Pocket",
	"ngpc": "Neo Geo Pocket Color", "odyssey2": "Odyssey 2", "pc": "DOS",
	"pc98": "PC-98", "pce": "PC Engine", "pcecd": "PC Engine CD",
	"pcengine": "PC Engine", "pico8": "PICO-8", "pokemini": "PokéMini",
	"ports": "Ports", "psp": "PSP", "psx": "PlayStation", "scummvm": "ScummVM",
	"sega32x": "Sega 32X", "segacd": "Sega CD", "sfc": "Super Famicom",
	"sg-1000": "SG-1000", "snes": "Super Nintendo", "spectrum": "ZX Spectrum",
	"supervision": "Supervision", "switch": "Switch", "vectrex": "Vectrex",
	"virtualboy": "Virtual Boy", "wonderswan": "WonderSwan",
	"wonderswancolor": "WonderSwan Color", "x68000": "X68000",
	"zx81": "ZX81", "zxspectrum": "ZX Spectrum", "doom": "Doom",
	"quake": "Quake", "openbor": "OpenBOR", "lowresnx": "LowRes NX",
	"snes-hacks": "SNES Hacks", "saturn": "Saturn", "3do": "3DO",
}

func DefaultConfig() *Config {
	return &Config{
		Name:            hostname(),
		DataDir:         defaultDataDir(),
		RomsRoot:        "",
		Protocol:        "tcp",
		Listen:          ":47820",
		Auth:            true,
		ScanIntervalSec: 900,
		StateRefreshSec: 60,
		MaxUploadMB:     128,
		Backups:         true,
		AllowControl:    true,
		Multicast: MulticastConfig{
			Group:       "239.255.42.99",
			Port:        47820,
			IntervalSec: 4,
		},
		ExcludeDirs: []string{
			"images", "media", "videos", "manuals", "downloaded_images",
			"gamelists", "bios", "system", ".git", "tmp", "cache",
		},
		SystemNames: map[string]string{},
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	if path != "" {
		if b, err := os.ReadFile(path); err == nil {
			if err := json.Unmarshal(b, cfg); err != nil {
				return nil, err
			}
			cfg.Path = path
		}
	}
	return cfg, nil
}

func (c *Config) Save() error {
	if c.Path == "" {
		return nil
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := c.Path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.Path)
}

// Ensure fills in the autodetected values and creates the directories that
// must exist before the daemon can serve requests.
func (c *Config) Ensure() {
	if c.Name == "" {
		c.Name = hostname()
	}
	for k, v := range defaultSystemNames {
		if _, ok := c.SystemNames[k]; !ok {
			c.SystemNames[k] = v
		}
	}

	c.DataDir = expandPath(c.DataDir)
	c.RomsRoot = expandPath(c.RomsRoot)
	c.RetroArchCfg = expandPath(c.RetroArchCfg)
	c.SpoolDir = expandPath(c.SpoolDir)
	if err := os.MkdirAll(c.DataDir, 0o755); err != nil {
		log.Printf("config: cannot create data dir %s: %v", c.DataDir, err)
	}

	if c.RomsRoot == "" || !isDir(c.RomsRoot) {
		for _, cand := range []string{"/roms", filepath.Join(c.DataDir, "roms")} {
			if isDir(cand) {
				c.RomsRoot = cand
				break
			}
		}
	}

	if c.SpoolDir == "" {
		if isDir("/dev/shm") {
			c.SpoolDir = "/dev/shm/multirk"
		} else {
			c.SpoolDir = filepath.Join(c.DataDir, "spool")
		}
	}
	if err := os.MkdirAll(c.SpoolDir, 0o755); err != nil {
		log.Printf("config: cannot create spool dir %s: %v", c.SpoolDir, err)
	}

	if len(c.StateDirs) == 0 || len(c.SaveDirs) == 0 {
		c.detectRetroArchDirs()
	}
	// ArkOS/RetroArch on these handhelds frequently writes savestates next to
	// the roms (/roms/<system>/<core>/foo.state). Include the rom root so the
	// sync index sees them without scanning every file.
	if c.RomsRoot != "" && isDir(c.RomsRoot) {
		c.StateDirs = appendUnique(c.StateDirs, c.RomsRoot)
		c.SaveDirs = appendUnique(c.SaveDirs, c.RomsRoot)
	}
	if c.ScanIntervalSec <= 0 {
		c.ScanIntervalSec = 900
	}
	if c.Protocol == "" {
		c.Protocol = "tcp"
	}
	if c.Protocol == "http" && c.HTTPListen == "" {
		c.HTTPListen = c.Listen
	}
	if c.StateRefreshSec <= 0 {
		c.StateRefreshSec = 60
	}
	if c.MaxUploadMB <= 0 {
		c.MaxUploadMB = 128
	}
	if c.Multicast.Group == "" {
		c.Multicast.Group = "239.255.42.99"
	}
	if c.Multicast.Port == 0 {
		c.Multicast.Port = 47820
	}
	if c.Multicast.IntervalSec <= 0 {
		c.Multicast.IntervalSec = 4
	}
}

func (c *Config) detectRetroArchDirs() {
	candidates := []string{}
	if c.RetroArchCfg != "" {
		candidates = append(candidates, c.RetroArchCfg)
	}
	home, _ := os.UserHomeDir()
	candidates = append(candidates,
		"/home/ark/.config/retroarch/retroarch.cfg",
		"/storage/.config/retroarch/retroarch.cfg",
		filepath.Join(home, ".config", "retroarch", "retroarch.cfg"),
	)

	seen := map[string]bool{}
	for _, rc := range candidates {
		rc = expandPath(rc)
		if rc == "" || seen[rc] {
			continue
		}
		seen[rc] = true
		b, err := os.ReadFile(rc)
		if err != nil {
			continue
		}
		dir := filepath.Dir(rc)
		log.Printf("config: reading retroarch settings from %s", rc)
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			kv := strings.SplitN(line, "=", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.TrimSpace(kv[0])
			val := strings.Trim(strings.TrimSpace(kv[1]), `"`)
			if val == "" {
				continue
			}
			switch key {
			case "savestate_directory":
				if strings.EqualFold(val, "default") {
					val = filepath.Join(dir, "states")
				}
				c.StateDirs = appendUnique(c.StateDirs, expandPath(val))
			case "savefile_directory":
				if strings.EqualFold(val, "default") {
					val = filepath.Join(dir, "saves")
				}
				c.SaveDirs = appendUnique(c.SaveDirs, expandPath(val))
			}
		}
		if c.RetroArchCfg == "" {
			c.RetroArchCfg = rc
		}
	}

	base := "/home/ark/.config/retroarch"
	if _, err := os.Stat("/storage/.config/retroarch"); err == nil {
		base = "/storage/.config/retroarch"
	}
	c.StateDirs = appendUnique(c.StateDirs,
		filepath.Join(base, "states"),
		"/home/ark/.config/retroarch/states",
		"/storage/.config/retroarch/states",
	)
	c.SaveDirs = appendUnique(c.SaveDirs,
		filepath.Join(base, "saves"),
		"/home/ark/.config/retroarch/saves",
		"/storage/.config/retroarch/saves",
	)
	if len(c.StateDirs) == 0 {
		log.Printf("config: no savestate directory found; set stateDirs in config.json")
	}
}

func (c *Config) isExcluded(name string) bool {
	name = strings.ToLower(name)
	if strings.HasPrefix(name, "bkp_") || strings.HasPrefix(name, "backup") {
		return true
	}
	for _, ex := range c.ExcludeDirs {
		if name == strings.ToLower(ex) {
			return true
		}
	}
	return false
}

func (c *Config) systemName(key string) string {
	if n, ok := c.SystemNames[strings.ToLower(key)]; ok && n != "" {
		return n
	}
	return strings.ToUpper(key[:1]) + key[1:]
}
