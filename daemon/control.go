package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Controls exposes the console hardware the Android app is allowed to touch.
// Everything is best-effort: ArkOS images differ in ALSA card names, backlight
// drivers and whether the user can write sysfs. Errors are returned to the
// client instead of crashing the agent.
type Controls struct {
	cfg *Config

	mu        sync.Mutex
	audioKind string // amixer | pactl
	audioName string
	backlight string // sysfs directory
}

type ControlState struct {
	Volume     *int   `json:"volume,omitempty"`
	Muted      *bool  `json:"muted,omitempty"`
	Brightness *int   `json:"brightness,omitempty"`
	Battery    *int   `json:"battery,omitempty"`
	Charging   *bool  `json:"charging,omitempty"`
	Audio      string `json:"audio,omitempty"`
	Backlight  string `json:"backlightDriver,omitempty"`
	Error      string `json:"error,omitempty"`
}

func NewControls(cfg *Config) *Controls { return &Controls{cfg: cfg} }

func (c *Controls) State() ControlState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stateLocked()
}

func (c *Controls) stateLocked() ControlState {
	var st ControlState
	if v, muted, backend, err := c.readVolumeLocked(); err == nil {
		st.Volume = &v
		st.Muted = &muted
		st.Audio = backend
	} else {
		st.Error = err.Error()
	}
	if v, driver, err := c.readBrightnessLocked(); err == nil {
		st.Brightness = &v
		st.Backlight = driver
	} else if st.Error == "" {
		st.Error = err.Error()
	}
	if pct, charging, ok := readBattery(); ok {
		st.Battery = &pct
		st.Charging = &charging
	}
	return st
}

func (c *Controls) SetVolume(v int) (ControlState, error) {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.writeVolumeLocked(v); err != nil {
		return c.stateLocked(), err
	}
	return c.stateLocked(), nil
}

func (c *Controls) SetMute(mute bool) (ControlState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.writeMuteLocked(mute); err != nil {
		return c.stateLocked(), err
	}
	return c.stateLocked(), nil
}

func (c *Controls) SetBrightness(v int) (ControlState, error) {
	if v < 1 {
		v = 1
	}
	if v > 100 {
		v = 100
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.writeBrightnessLocked(v); err != nil {
		return c.stateLocked(), err
	}
	return c.stateLocked(), nil
}

// ---------------------------------------------------------------- audio ----

var reMixerControl = regexp.MustCompile(`'([^']+)'`)
var rePercent = regexp.MustCompile(`\[(\d+)%\]`)

func (c *Controls) detectAudioLocked() error {
	if c.audioKind != "" {
		return nil
	}
	if c.cfg.AudioControl != "" {
		if _, err := exec.LookPath("amixer"); err == nil {
			c.audioKind, c.audioName = "amixer", c.cfg.AudioControl
			return nil
		}
	}
	if _, err := exec.LookPath("amixer"); err == nil {
		if out, err := runCmd("amixer", "scontrols"); err == nil {
			if name := pickMixer(string(out)); name != "" {
				c.audioKind, c.audioName = "amixer", name
				return nil
			}
		}
		// ArkOS usually exposes Playback; harmless if it does not.
		c.audioKind, c.audioName = "amixer", "Playback"
		return nil
	}
	if _, err := exec.LookPath("pactl"); err == nil {
		c.audioKind = "pactl"
		return nil
	}
	return errors.New("no hay amixer ni pactl en la consola")
}

func pickMixer(scontrols string) string {
	names := reMixerControl.FindAllStringSubmatch(scontrols, -1)
	prefs := []string{"master", "playback", "pcm", "speaker", "headphone", "digital", "lineout"}
	for _, pref := range prefs {
		for _, m := range names {
			if strings.EqualFold(strings.TrimSpace(m[1]), pref) {
				return m[1]
			}
		}
	}
	if len(names) > 0 {
		return names[0][1]
	}
	return ""
}

func (c *Controls) readVolumeLocked() (int, bool, string, error) {
	if err := c.detectAudioLocked(); err != nil {
		return 0, false, "", err
	}
	if c.audioKind == "pactl" {
		out, err := runCmd("pactl", "get-sink-volume", "@DEFAULT_SINK@")
		if err != nil {
			return 0, false, "pactl", err
		}
		m := rePercent.FindStringSubmatch(string(out))
		if m == nil {
			return 0, false, "pactl", errors.New("volumen no legible")
		}
		v, _ := strconv.Atoi(m[1])
		muteOut, _ := runCmd("pactl", "get-sink-mute", "@DEFAULT_SINK@")
		muted := strings.Contains(strings.ToLower(string(muteOut)), "yes")
		return v, muted, "pactl", nil
	}
	out, err := runCmd("amixer", "sget", c.audioName)
	if err != nil {
		// Some cards need an explicit card number.
		out, err = runCmd("amixer", "-c", "0", "sget", c.audioName)
		if err != nil {
			return 0, false, "amixer", err
		}
	}
	text := string(out)
	m := rePercent.FindStringSubmatch(text)
	if m == nil {
		return 0, false, "amixer", fmt.Errorf("control %q sin porcentaje", c.audioName)
	}
	v, _ := strconv.Atoi(m[1])
	muted := strings.Contains(strings.ToLower(text), "[off]")
	return v, muted, "amixer", nil
}

func (c *Controls) writeVolumeLocked(v int) error {
	if err := c.detectAudioLocked(); err != nil {
		return err
	}
	if c.audioKind == "pactl" {
		_, err := runCmd("pactl", "set-sink-volume", "@DEFAULT_SINK@", fmt.Sprintf("%d%%", v))
		return err
	}
	if _, err := runCmd("amixer", "sset", c.audioName, fmt.Sprintf("%d%%", v)); err != nil {
		_, err = runCmd("amixer", "-c", "0", "sset", c.audioName, fmt.Sprintf("%d%%", v))
		return err
	}
	return nil
}

func (c *Controls) writeMuteLocked(mute bool) error {
	if err := c.detectAudioLocked(); err != nil {
		return err
	}
	if c.audioKind == "pactl" {
		arg := "0"
		if mute {
			arg = "1"
		}
		_, err := runCmd("pactl", "set-sink-mute", "@DEFAULT_SINK@", arg)
		return err
	}
	arg := "unmute"
	if mute {
		arg = "mute"
	}
	if _, err := runCmd("amixer", "sset", c.audioName, arg); err != nil {
		_, err = runCmd("amixer", "-c", "0", "sset", c.audioName, arg)
		return err
	}
	return nil
}

// ----------------------------------------------------------- brightness ----

func (c *Controls) detectBacklightLocked() error {
	if c.backlight != "" {
		return nil
	}
	if c.cfg.Backlight != "" {
		dir := filepath.Join("/sys/class/backlight", c.cfg.Backlight)
		if isFile(filepath.Join(dir, "brightness")) {
			c.backlight = dir
			return nil
		}
	}
	entries, err := os.ReadDir("/sys/class/backlight")
	if err != nil {
		return errors.New("sin /sys/class/backlight")
	}
	for _, prefer := range []string{"backlight", "pwm-backlight", "rk817-backlight", "lvds-backlight"} {
		for _, e := range entries {
			if strings.EqualFold(e.Name(), prefer) {
				dir := filepath.Join("/sys/class/backlight", e.Name())
				if isFile(filepath.Join(dir, "brightness")) {
					c.backlight = dir
					return nil
				}
			}
		}
	}
	for _, e := range entries {
		dir := filepath.Join("/sys/class/backlight", e.Name())
		if isFile(filepath.Join(dir, "brightness")) {
			c.backlight = dir
			return nil
		}
	}
	return errors.New("no se encontró control de brillo")
}

func (c *Controls) readBrightnessLocked() (int, string, error) {
	if err := c.detectBacklightLocked(); err != nil {
		return 0, "", err
	}
	max, err := readIntFile(filepath.Join(c.backlight, "max_brightness"))
	if err != nil || max <= 0 {
		return 0, "", errors.New("max_brightness ilegible")
	}
	cur, err := readIntFile(filepath.Join(c.backlight, "brightness"))
	if err != nil {
		return 0, "", err
	}
	pct := int(float64(cur)*100/float64(max) + 0.5)
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct, filepath.Base(c.backlight), nil
}

func (c *Controls) writeBrightnessLocked(pct int) error {
	if err := c.detectBacklightLocked(); err != nil {
		return err
	}
	max, err := readIntFile(filepath.Join(c.backlight, "max_brightness"))
	if err != nil || max <= 0 {
		return errors.New("max_brightness ilegible")
	}
	raw := int(float64(max)*float64(pct)/100.0 + 0.5)
	if raw < 1 {
		raw = 1
	}
	if raw > max {
		raw = max
	}
	err = os.WriteFile(filepath.Join(c.backlight, "brightness"), []byte(strconv.Itoa(raw)), 0o644)
	if err == nil {
		return nil
	}
	// Debian/Arch images often ship brightnessctl with a udev rule already.
	if _, lookErr := exec.LookPath("brightnessctl"); lookErr == nil {
		if _, runErr := runCmd("brightnessctl", "set", fmt.Sprintf("%d%%", pct)); runErr == nil {
			return nil
		}
	}
	return fmt.Errorf("sin permiso para escribir brillo (añade el usuario al grupo video o usa brightnessctl): %w", err)
}

// -------------------------------------------------------------- battery ----

func readBattery() (int, bool, bool) {
	entries, err := os.ReadDir("/sys/class/power_supply")
	if err != nil {
		return 0, false, false
	}
	for _, e := range entries {
		dir := filepath.Join("/sys/class/power_supply", e.Name())
		pct, err := readIntFile(filepath.Join(dir, "capacity"))
		if err != nil {
			continue
		}
		charging := false
		if b, err := os.ReadFile(filepath.Join(dir, "status")); err == nil {
			s := strings.ToLower(strings.TrimSpace(string(b)))
			charging = s == "charging" || s == "full"
		}
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		return pct, charging, true
	}
	return 0, false, false
}

// ---------------------------------------------------------------- helpers ---

func readIntFile(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, err
	}
	return n, nil
}

func runCmd(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}
