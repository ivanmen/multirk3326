package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type StateFile struct {
	Path   string
	Size   int64
	Mtime  int64
	Suffix string
}

type Artifact struct {
	Kind  string `json:"kind"`
	Slot  string `json:"slot,omitempty"`
	File  string `json:"file"`
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"`
	Thumb bool   `json:"thumb,omitempty"`
}

var stateSlots = []string{"auto", "0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}

var stateSuffixes = buildStateSuffixes()

func buildStateSuffixes() []string {
	out := []string{".state.auto.png", ".state.auto"}
	for i := 1; i <= 9; i++ {
		out = append(out, fmt.Sprintf(".state%d.png", i))
	}
	out = append(out, ".state.png")
	for i := 1; i <= 9; i++ {
		out = append(out, fmt.Sprintf(".state%d", i))
	}
	out = append(out, ".state", ".srm", ".rtc")
	return out
}

// splitStateName recognises every file name RetroArch writes for a content
// file: Foo.state, Foo.state.auto, Foo.state1..9, Foo.srm, Foo.rtc and the
// matching .png thumbnails.
func splitStateName(name string) (base, suffix string, ok bool) {
	lower := strings.ToLower(name)
	for _, s := range stateSuffixes {
		if len(name) > len(s) && strings.HasSuffix(lower, s) {
			return name[:len(name)-len(s)], s, true
		}
	}
	return "", "", false
}

func slotSuffix(slot string) string {
	switch slot {
	case "auto":
		return ".state.auto"
	case "", "0":
		return ".state"
	}
	if n, err := strconv.Atoi(slot); err == nil && n >= 1 && n <= 9 {
		return ".state" + strconv.Itoa(n)
	}
	return ""
}

func slotThumbSuffix(slot string) string {
	s := slotSuffix(slot)
	if s == "" {
		return ""
	}
	return s + ".png"
}

func artifactSuffix(kind, slot string) (string, error) {
	switch kind {
	case "", "state":
		s := slotSuffix(slot)
		if s == "" {
			return "", fmt.Errorf("slot inválido %q", slot)
		}
		return s, nil
	case "sram":
		return ".srm", nil
	case "rtc":
		return ".rtc", nil
	}
	return "", fmt.Errorf("kind inválido %q", kind)
}

func (ix *Index) stateRoots() []string {
	roots := make([]string, 0, len(ix.cfg.StateDirs)+len(ix.cfg.SaveDirs))
	seen := map[string]bool{}
	for _, d := range append(append([]string{}, ix.cfg.StateDirs...), ix.cfg.SaveDirs...) {
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		roots = append(roots, d)
	}
	return roots
}

// buildStates walks only the save folders (a few thousand files at most) and
// indexes them by content base name. Runs in RAM; never writes to the SD.
func (ix *Index) buildStates() map[string]map[string]StateFile {
	out := make(map[string]map[string]StateFile)

	add := func(p string, d fs.DirEntry) {
		base, suffix, ok := splitStateName(d.Name())
		if !ok {
			return
		}
		info, err := d.Info()
		if err != nil {
			return
		}
		key := strings.ToLower(base)
		m := out[key]
		if m == nil {
			m = make(map[string]StateFile, 4)
			out[key] = m
		}
		sf := StateFile{Path: p, Size: info.Size(), Mtime: info.ModTime().Unix(), Suffix: suffix}
		if prev, exists := m[suffix]; !exists || sf.Mtime > prev.Mtime {
			m[suffix] = sf
		}
	}

	for _, root := range ix.stateRoots() {
		root := root
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				if p == root {
					return nil
				}
				if strings.HasPrefix(name, ".") || name == "thumbnails" {
					return filepath.SkipDir
				}
				rel, _ := filepath.Rel(root, p)
				if strings.Count(rel, string(os.PathSeparator)) >= 2 {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasPrefix(name, ".") {
				return nil
			}
			add(p, d)
			return nil
		})
	}
	return out
}

func applyStateFlags(g *Game, states map[string]map[string]StateFile) {
	g.HasState = false
	g.HasSram = false
	g.Artifacts = 0
	g.StateMtime = 0
	g.SramMtime = 0

	m := states[strings.ToLower(g.Base)]
	if m == nil {
		return
	}
	for _, slot := range stateSlots {
		sf, ok := m[slotSuffix(slot)]
		if !ok {
			continue
		}
		g.HasState = true
		g.Artifacts++
		if sf.Mtime > g.StateMtime {
			g.StateMtime = sf.Mtime
		}
		if _, ok := m[slotThumbSuffix(slot)]; ok {
			g.Artifacts++
		}
	}
	if sf, ok := m[".srm"]; ok {
		g.HasSram = true
		g.SramMtime = sf.Mtime
		g.Artifacts++
	}
	if _, ok := m[".rtc"]; ok {
		g.Artifacts++
	}
}

func buildArtifacts(g *Game, states map[string]map[string]StateFile) []Artifact {
	m := states[strings.ToLower(g.Base)]
	if m == nil {
		return nil
	}
	var out []Artifact
	for _, slot := range stateSlots {
		sf, ok := m[slotSuffix(slot)]
		if !ok {
			continue
		}
		a := Artifact{
			Kind:  "state",
			Slot:  slot,
			File:  filepath.Base(sf.Path),
			Size:  sf.Size,
			Mtime: sf.Mtime,
		}
		if _, ok := m[slotThumbSuffix(slot)]; ok {
			a.Thumb = true
		}
		out = append(out, a)
	}
	if sf, ok := m[".srm"]; ok {
		out = append(out, Artifact{Kind: "sram", File: filepath.Base(sf.Path), Size: sf.Size, Mtime: sf.Mtime})
	}
	if sf, ok := m[".rtc"]; ok {
		out = append(out, Artifact{Kind: "rtc", File: filepath.Base(sf.Path), Size: sf.Size, Mtime: sf.Mtime})
	}
	return out
}

func (ix *Index) Artifacts(g *Game) []Artifact {
	ix.mu.RLock()
	states := ix.states
	ix.mu.RUnlock()
	return buildArtifacts(g, states)
}

func (ix *Index) findArtifact(g *Game, kind, slot string) (StateFile, bool) {
	suffix, err := artifactSuffix(kind, slot)
	if err != nil {
		return StateFile{}, false
	}
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	m := ix.states[strings.ToLower(g.Base)]
	if m == nil {
		return StateFile{}, false
	}
	sf, ok := m[suffix]
	return sf, ok
}

// dirsFor picks where a new artifact should be created.
func (ix *Index) dirsFor(kind string) []string {
	if kind == "sram" || kind == "rtc" {
		if len(ix.cfg.SaveDirs) > 0 {
			return ix.cfg.SaveDirs
		}
	}
	return ix.cfg.StateDirs
}

func (ix *Index) TargetPath(g *Game, kind, slot string) (string, error) {
	if g == nil {
		return "", errors.New("juego desconocido")
	}
	if g.Base == "" || strings.ContainsAny(g.Base, `/\`) || g.Base == "." || g.Base == ".." {
		return "", errors.New("nombre de rom inválido")
	}
	suffix, err := artifactSuffix(kind, slot)
	if err != nil {
		return "", err
	}
	dirs := ix.dirsFor(kind)
	if len(dirs) == 0 {
		return "", errors.New("no hay carpeta de estados configurada")
	}
	return filepath.Join(dirs[0], g.Base+suffix), nil
}

// ResolveArtifact returns fresh metadata: the in-memory index is used first,
// then a direct stat so a state written by RetroArch seconds ago is visible
// without waiting for the next refresh.
func (ix *Index) ResolveArtifact(g *Game, kind, slot string) (StateFile, bool) {
	if sf, ok := ix.findArtifact(g, kind, slot); ok {
		if st, err := os.Stat(sf.Path); err == nil && !st.IsDir() {
			return StateFile{Path: sf.Path, Size: st.Size(), Mtime: st.ModTime().Unix(), Suffix: sf.Suffix}, true
		}
	}
	p, err := ix.TargetPath(g, kind, slot)
	if err != nil {
		return StateFile{}, false
	}
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return StateFile{Path: p, Size: st.Size(), Mtime: st.ModTime().Unix()}, true
	}
	return StateFile{}, false
}

// WriteArtifact streams an upload to the RAM spool, verifies its checksum and
// only then touches the SD card, atomically. A previous file is kept once as
// <name>.multirk.bak.
func (ix *Index) WriteArtifact(g *Game, kind, slot string, r io.Reader, maxBytes int64, wantSha string, mtime int64, backup bool) (int64, string, string, error) {
	target, err := ix.TargetPath(g, kind, slot)
	if err != nil {
		return 0, "", "", err
	}
	size, sum, err := writeAtomic(ix.cfg.SpoolDir, target, r, maxBytes, wantSha, mtime, backup)
	if err != nil {
		return size, sum, target, err
	}
	// Refresh in the background; the response does not need to wait for it.
	go ix.RefreshStates()
	return size, sum, target, nil
}

func writeAtomic(spoolDir, target string, r io.Reader, maxBytes int64, wantSha string, mtime int64, backup bool) (int64, string, error) {
	if spoolDir == "" {
		spoolDir = os.TempDir()
	}
	tmp, err := os.CreateTemp(spoolDir, "multirk-up-*")
	if err != nil {
		return 0, "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(r, maxBytes+1))
	if copyErr != nil {
		tmp.Close()
		return n, "", copyErr
	}
	if n > maxBytes {
		tmp.Close()
		return n, "", fmt.Errorf("el archivo supera el límite de %d bytes", maxBytes)
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if wantSha != "" && !strings.EqualFold(strings.TrimSpace(wantSha), sum) {
		tmp.Close()
		return n, sum, fmt.Errorf("checksum no coincide")
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return n, sum, err
	}
	if err := tmp.Close(); err != nil {
		return n, sum, err
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return n, sum, err
	}
	if backup {
		backupOnce(target)
	}
	if err := os.Rename(tmpName, target); err != nil {
		// Most likely EXDEV: spool is tmpfs and the SD is another filesystem.
		if cerr := copyIntoPlace(tmpName, target); cerr != nil {
			return n, sum, fmt.Errorf("no se pudo mover a su sitio: %v (fallback: %v)", err, cerr)
		}
	}
	if mtime > 0 {
		t := time.Unix(mtime, 0)
		_ = os.Chtimes(target, t, t)
	}
	fsyncDir(filepath.Dir(target))
	return n, sum, nil
}

// commitTemp moves an already-received temp file into place atomically. Used
// by the TCP protocol, where the upload is streamed into the spool first.
func commitTemp(tmpName, target string, mtime int64, backup bool) (int64, error) {
	st, err := os.Stat(tmpName)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return 0, err
	}
	if backup {
		backupOnce(target)
	}
	if err := os.Rename(tmpName, target); err != nil {
		if cerr := copyIntoPlace(tmpName, target); cerr != nil {
			return 0, fmt.Errorf("no se pudo mover a su sitio: %v (fallback: %v)", err, cerr)
		}
	}
	if mtime > 0 {
		t := time.Unix(mtime, 0)
		_ = os.Chtimes(target, t, t)
	}
	fsyncDir(filepath.Dir(target))
	return st.Size(), nil
}

func copyIntoPlace(src, target string) error {	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	dir := filepath.Dir(target)
	out, err := os.CreateTemp(dir, ".multirk-*")
	if err != nil {
		return err
	}
	tmpName := out.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, target)
}

func backupOnce(target string) {
	if !isFile(target) {
		return
	}
	bak := target + ".multirk.bak"
	_ = os.Remove(bak)
	_ = os.Rename(target, bak)
}

func fsyncDir(dir string) {
	if runtime.GOOS == "windows" {
		return
	}
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	defer d.Close()
	_ = d.Sync()
}
