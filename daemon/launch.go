package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	esSystemsCfg = "/etc/emulationstation/es_systems.cfg"
	gameUnit     = "multirk-game"
)

type esSystem struct {
	Name      string `xml:"name"`
	Command   string `xml:"command"`
	Emulators []struct {
		Name  string   `xml:"name,attr"`
		Cores []string `xml:"cores>core"`
	} `xml:"emulators>emulator"`
}

type esSystemList struct {
	Systems []esSystem `xml:"system"`
}

// Launcher runs a game the way EmulationStation would. The game runs in its
// own transient systemd unit (outside the agent's cgroup, so the agent's
// memory/CPU limits do not apply): ES is stopped, the system's command runs
// with the ROM, then performance is restored and ES is started again. That
// last part runs even if the game crashes or is closed from the phone.
type Launcher struct {
	mu   sync.Mutex
	game string
	rom  string
}

type LaunchState struct {
	Running bool   `json:"running"`
	Game    string `json:"game,omitempty"`
}

func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// buildLaunchCommand fills the es_systems command template for a ROM.
func buildLaunchCommand(cfgXML []byte, system, rom string) (string, error) {
	var list esSystemList
	dec := xml.NewDecoder(strings.NewReader(string(cfgXML)))
	dec.Strict = false
	if err := dec.Decode(&list); err != nil {
		return "", err
	}
	for _, sy := range list.Systems {
		if !strings.EqualFold(sy.Name, system) {
			continue
		}
		emu, core := "", ""
		if len(sy.Emulators) > 0 {
			emu = sy.Emulators[0].Name
			if len(sy.Emulators[0].Cores) > 0 {
				core = sy.Emulators[0].Cores[0]
			}
		}
		base := strings.TrimSuffix(filepath.Base(rom), filepath.Ext(rom))
		cmd := strings.TrimSpace(sy.Command)
		r := strings.NewReplacer(
			"%ROM%", shQuote(rom),
			"%BASENAME%", shQuote(base),
			"%EMULATOR%", emu,
			"%CORE%", core,
			"%GOVERNOR%", "",
		)
		return r.Replace(cmd), nil
	}
	return "", fmt.Errorf("sistema %q sin comando de lanzamiento", system)
}

func gameUnitActive() bool {
	return exec.Command("systemctl", "is-active", "--quiet", gameUnit).Run() == nil
}

func (l *Launcher) State() LaunchState {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !gameUnitActive() {
		return LaunchState{}
	}
	return LaunchState{Running: true, Game: l.game}
}

const launchScript = `export TERM=linux XDG_RUNTIME_DIR=/run/user/$(id -u)/ SDL_ASSERT=always_ignore
sudo -n chmod 666 /dev/tty1
sudo -n systemctl stop emulationstation
sleep 1
bash -c "$0"
sudo -n perfnorm
sudo -n systemctl start emulationstation`

func (l *Launcher) Launch(romsRoot string, g *Game) error {
	rom := filepath.Join(romsRoot, g.System, filepath.FromSlash(g.RelPath))
	if _, err := os.Stat(rom); err != nil {
		return errors.New("la ROM no existe en la consola")
	}
	data, err := os.ReadFile(esSystemsCfg)
	if err != nil {
		return errors.New("no encuentro es_systems.cfg")
	}
	cmdline, err := buildLaunchCommand(data, g.System, rom)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if gameUnitActive() {
		return errors.New("ya hay un juego en marcha")
	}
	out, err := exec.Command("sudo", "-n", "systemd-run", "--unit="+gameUnit, "--collect",
		"--uid=ark", "--property=Nice=0",
		"bash", "-c", launchScript, cmdline).CombinedOutput()
	if err != nil {
		return fmt.Errorf("no se pudo lanzar: %s", strings.TrimSpace(string(out)))
	}
	l.game, l.rom = g.Name, rom
	return nil
}

// gamePids returns the processes of the running game, never the unit's own
// wrapper script (its command line also contains the ROM path, and it must
// survive to bring ES back).
func gamePids(rom string) []string {
	out, _ := exec.Command("pgrep", "-f", "-a", "--", rom).Output()
	var pids []string
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.SplitN(strings.TrimSpace(line), " ", 2)
		if len(f) < 2 || strings.Contains(f[1], "systemctl stop emulationstation") {
			continue
		}
		pids = append(pids, f[0])
	}
	return pids
}

// Close ends the running game; the unit's script then brings ES back.
func (l *Launcher) Close() error {
	l.mu.Lock()
	rom := l.rom
	l.mu.Unlock()
	if rom == "" || !gameUnitActive() {
		return errors.New("no hay ningún juego en marcha")
	}
	kill := func(sig string) {
		for _, p := range gamePids(rom) {
			_ = exec.Command("kill", "-"+sig, p).Run()
		}
	}
	kill("TERM")
	time.Sleep(3 * time.Second)
	kill("KILL")
	return nil
}
