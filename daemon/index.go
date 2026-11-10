package main

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type System struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Games     int    `json:"games"`
	WithState int    `json:"withState"`
}

type Game struct {
	ID      string `json:"id"`
	System  string `json:"system"`
	RelPath string `json:"relPath"`
	File    string `json:"file"`
	Base    string `json:"-"`
	Ext     string `json:"ext"`
	Name    string `json:"name"`
	// MatchKey correlates the same game across two consoles even when the
	// folder layout differs. system|basename (no extension), lowercased.
	MatchKey    string  `json:"matchKey"`
	Size        int64   `json:"size"`
	Mtime       int64   `json:"mtime"`
	Missing     bool    `json:"missing"`
	Desc        string  `json:"desc,omitempty"`
	Rating      float64 `json:"rating,omitempty"`
	Developer   string  `json:"developer,omitempty"`
	Publisher   string  `json:"publisher,omitempty"`
	Genre       string  `json:"genre,omitempty"`
	Players     string  `json:"players,omitempty"`
	Releasedate string  `json:"releasedate,omitempty"`

	// Media live on the SD but are only ever read, and only on demand.
	Image   string `json:"-"`
	Thumb   string `json:"-"`
	Marquee string `json:"-"`
	Video   string `json:"-"`

	HasImage  bool `json:"hasImage"`
	Scrapable bool `json:"scrapable"`
	HasVideo  bool `json:"hasVideo"`
	HasState  bool `json:"hasState"`
	StateMtime int64 `json:"stateMtime"`
	HasSram   bool `json:"hasSram"`
	SramMtime int64 `json:"sramMtime"`
	Artifacts int  `json:"artifacts"`
}

type Snapshot struct {
	Systems  []System            `json:"systems"`
	Games    []*Game             `json:"games"`
	ByID     map[string]*Game    `json:"-"`
	BySystem map[string][]*Game  `json:"-"`
	Built    time.Time           `json:"built"`
}

type gamelistEntry struct {
	Kind        string
	Path        string
	Name        string
	Desc        string
	Image       string
	Thumbnail   string
	Marquee     string
	Video       string
	Rating      string
	Releasedate string
	Developer   string
	Publisher   string
	Genre       string
	Players     string
}

func (e *gamelistEntry) set(field, value string) {
	switch field {
	case "path":
		e.Path = value
	case "name":
		e.Name = value
	case "desc":
		e.Desc = value
	case "image":
		e.Image = value
	case "thumbnail":
		e.Thumbnail = value
	case "marquee":
		e.Marquee = value
	case "video":
		e.Video = value
	case "rating":
		e.Rating = value
	case "releasedate":
		e.Releasedate = value
	case "developer":
		e.Developer = value
	case "publisher":
		e.Publisher = value
	case "genre":
		e.Genre = value
	case "players":
		e.Players = value
	}
}

// parseGamelist streams an EmulationStation gamelist.xml. Streaming matters:
// some ArkOS builds ship gamelists of tens of MB on a 1 GB RAM device.
func parseGamelist(path string) ([]*gamelistEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := xml.NewDecoder(bufio.NewReaderSize(f, 1<<16))
	var out []*gamelistEntry
	var cur *gamelistEntry
	var field string
	var sb strings.Builder

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Keep whatever we parsed so far: a truncated gamelist should not
			// hide the whole system.
			return out, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := strings.ToLower(t.Name.Local)
			if name == "game" || name == "folder" {
				cur = &gamelistEntry{Kind: name}
				field = ""
				continue
			}
			if cur != nil {
				field = name
				sb.Reset()
			}
		case xml.CharData:
			if cur != nil && field != "" {
				sb.Write(t)
			}
		case xml.EndElement:
			if cur == nil {
				continue
			}
			name := strings.ToLower(t.Name.Local)
			if name == "game" || name == "folder" {
				if cur.Kind == "game" {
					out = append(out, cur)
				}
				cur = nil
				field = ""
				continue
			}
			if name == field {
				cur.set(field, strings.TrimSpace(sb.String()))
				field = ""
			}
		}
	}
	return out, nil
}

// ---------------------------------------------------------------- index ----

type Index struct {
	cfg *Config

	scanMu sync.Mutex // one rom scan at a time
	mu     sync.RWMutex
	snap   *Snapshot
	states map[string]map[string]StateFile
	statesAt time.Time
	scanning bool
	lastErr  string
}

func NewIndex(cfg *Config) *Index {
	return &Index{cfg: cfg, states: map[string]map[string]StateFile{}}
}

func (ix *Index) Config() *Config { return ix.cfg }

func (ix *Index) Snapshot() *Snapshot {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return ix.snap
}

func (ix *Index) Scanning() bool {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return ix.scanning
}

func (ix *Index) LastError() string {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return ix.lastErr
}

func (ix *Index) StatesAt() time.Time {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return ix.statesAt
}

// Scan walks the rom folders (read-only: list + stat) and the state folders,
// then publishes a new immutable snapshot. Nothing is written to the SD card.
func (ix *Index) Scan() error {
	ix.scanMu.Lock()
	defer ix.scanMu.Unlock()

	ix.mu.Lock()
	ix.scanning = true
	ix.mu.Unlock()

	scanErr := ix.scanLocked()

	ix.mu.Lock()
	ix.scanning = false
	if scanErr != nil {
		ix.lastErr = scanErr.Error()
	} else {
		ix.lastErr = ""
	}
	ix.mu.Unlock()
	return scanErr
}

// ScanSystem re-reads only one system's folder (its roms and gamelists) and
// publishes a new snapshot with the other systems untouched. This is what a
// per-system refresh, a scrape, an edit or a delete use: the rest of the SD
// card is not read again.
func (ix *Index) ScanSystem(key string) error {
	ix.mu.RLock()
	old, states := ix.snap, ix.states
	ix.mu.RUnlock()
	if old == nil {
		return ix.Scan()
	}

	ix.scanMu.Lock()
	defer ix.scanMu.Unlock()

	fresh := ix.scanSystem(key)
	games := make([]*Game, 0, len(old.Games)+len(fresh))
	for _, g := range old.Games {
		if g.System != key {
			games = append(games, g)
		}
	}
	games = append(games, fresh...)
	snap := ix.newSnapshot(games, states)

	ix.mu.Lock()
	ix.snap = snap
	ix.mu.Unlock()
	log.Printf("scan: sistema %s (%d juegos)", key, len(fresh))
	return nil
}

func (ix *Index) scanLocked() error {
	if ix.cfg.RomsRoot == "" || !isDir(ix.cfg.RomsRoot) {
		return fmt.Errorf("roms root %q not found", ix.cfg.RomsRoot)
	}
	dirents, err := os.ReadDir(ix.cfg.RomsRoot)
	if err != nil {
		return err
	}

	states := ix.buildStates()

	var names []string
	for _, d := range dirents {
		if !d.IsDir() {
			continue
		}
		key := d.Name()
		if strings.HasPrefix(key, ".") || ix.cfg.isExcluded(key) {
			continue
		}
		names = append(names, key)
	}
	sort.Strings(names)

	var games []*Game
	for _, key := range names {
		games = append(games, ix.scanSystem(key)...)
	}

	snap := ix.newSnapshot(games, states)

	ix.mu.Lock()
	ix.snap = snap
	ix.states = states
	ix.statesAt = time.Now()
	ix.mu.Unlock()

	log.Printf("scan: %d systems, %d games, %d with state (%.0f ms)",
		len(snap.Systems), len(snap.Games), countWithState(snap.Games),
		float64(time.Since(snap.Built).Microseconds())/1000)
	return nil
}

// RefreshStates only re-reads the (small) savestate folders. Cheap enough to
// run every minute and keeps "has state" badges accurate without rescanning
// thousands of roms.
func (ix *Index) RefreshStates() {
	if !ix.scanMu.TryLock() {
		return
	}
	defer ix.scanMu.Unlock()

	states := ix.buildStates()

	ix.mu.Lock()
	ix.states = states
	ix.statesAt = time.Now()
	old := ix.snap
	var snap *Snapshot
	if old != nil {
		snap = ix.newSnapshot(old.Games, states)
		ix.snap = snap
	}
	ix.mu.Unlock()
}

func (ix *Index) scanSystem(sysKey string) []*Game {
	sysDir := filepath.Join(ix.cfg.RomsRoot, sysKey)
	var games []*Game

	_ = filepath.WalkDir(sysDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if p == sysDir {
				return nil
			}
			if strings.HasPrefix(name, ".") || ix.cfg.isExcluded(name) {
				return filepath.SkipDir
			}
			rel, _ := filepath.Rel(sysDir, p)
			if strings.Count(rel, string(os.PathSeparator)) >= 3 {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") || isNonRomFile(name) {
			return nil
		}
		if exts, ok := onlyExtensions[strings.ToLower(sysKey)]; ok &&
			!exts[strings.ToLower(filepath.Ext(name))] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(sysDir, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		base, ext := splitBaseExt(name)
		games = append(games, &Game{
			ID:      gameID(sysKey, rel),
			System:  sysKey,
			RelPath: rel,
			File:    name,
			Base:    base,
			Ext:     ext,
			Name:    prettyName(base),
			Size:    info.Size(),
			Mtime:   info.ModTime().Unix(),
		})
		return nil
	})

	ix.mergeNestedGamelists(sysDir, sysKey, &games)
	ix.mergeGamelistAt(sysDir, sysDir, sysKey, &games, false)
	for _, g := range games {
		g.MatchKey = strings.ToLower(g.System) + "|" + strings.ToLower(g.Base)
	}
	return games
}

// mergeNestedGamelists reads gamelist.xml files that live in sub-folders
// (ES supports them, e.g. ngp/Featured Games). Their paths and media are
// relative to that sub-folder. The system's own gamelist is merged after, so
// it keeps priority.
func (ix *Index) mergeNestedGamelists(sysDir, sysKey string, games *[]*Game) {
	ents, err := os.ReadDir(sysDir)
	if err != nil {
		return
	}
	for _, d := range ents {
		if !d.IsDir() || strings.HasPrefix(d.Name(), ".") || ix.cfg.isExcluded(d.Name()) {
			continue
		}
		sub := filepath.Join(sysDir, d.Name())
		if _, err := os.Stat(filepath.Join(sub, "gamelist.xml")); err == nil {
			ix.mergeGamelistAt(sysDir, sub, sysKey, games, true)
		}
	}
}

func (ix *Index) mergeGamelist(sysDir, sysKey string, games *[]*Game) {
	ix.mergeGamelistAt(sysDir, sysDir, sysKey, games, false)
}

func (ix *Index) mergeGamelistAt(sysDir, dir, sysKey string, games *[]*Game, nested bool) {
	entries, err := parseGamelist(filepath.Join(dir, "gamelist.xml"))
	if err != nil && len(entries) == 0 {
		return
	}
	if len(entries) == 0 {
		return
	}

	byRel := make(map[string]*Game, len(*games))
	byBase := make(map[string]*Game, len(*games))
	dupBase := make(map[string]bool)
	for _, g := range *games {
		byRel[strings.ToLower(g.RelPath)] = g
		lb := strings.ToLower(g.Base)
		if byBase[lb] != nil {
			dupBase[lb] = true
		}
		byBase[lb] = g
	}

	for _, e := range entries {
		rel := resolveGamelistPath(dir, e.Path)
		if nested && rel != "" {
			abs := filepath.Join(dir, filepath.FromSlash(rel))
			if r2, err := filepath.Rel(sysDir, abs); err == nil {
				rel = filepath.ToSlash(r2)
			}
		}
		var g *Game
		if rel != "" {
			g = byRel[strings.ToLower(rel)]
		}
		if g == nil && e.Path != "" {
			p := filepath.FromSlash(strings.TrimSpace(e.Path))
			lb := strings.ToLower(strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)))
			if !dupBase[lb] {
				g = byBase[lb]
			}
		}
		if g == nil {
			if rel == "" || nested {
				continue
			}
			base, ext := splitBaseExt(filepath.Base(rel))
			g = &Game{
				ID:      gameID(sysKey, rel),
				System:  sysKey,
				RelPath: rel,
				File:    filepath.Base(rel),
				Base:    base,
				Ext:     ext,
				Missing: true,
			}
			*games = append(*games, g)
			byRel[strings.ToLower(rel)] = g
		}
		if e.Name != "" {
			g.Name = e.Name
		}
		if e.Desc != "" {
			g.Desc = e.Desc
		}
		g.Developer = firstNonEmpty(e.Developer, g.Developer)
		g.Publisher = firstNonEmpty(e.Publisher, g.Publisher)
		g.Genre = firstNonEmpty(e.Genre, g.Genre)
		g.Players = firstNonEmpty(e.Players, g.Players)
		g.Releasedate = firstNonEmpty(e.Releasedate, g.Releasedate)
		if e.Rating != "" {
			if f, err := strconv.ParseFloat(strings.TrimSpace(e.Rating), 64); err == nil {
				g.Rating = f
			}
		}
		g.Image = firstNonEmpty(resolveMedia(dir, e.Image), g.Image)
		g.Thumb = firstNonEmpty(resolveMedia(dir, e.Thumbnail), g.Thumb)
		g.Marquee = firstNonEmpty(resolveMedia(dir, e.Marquee), g.Marquee)
		g.Video = firstNonEmpty(resolveMedia(dir, e.Video), g.Video)
		if g.Name == "" {
			g.Name = prettyName(g.Base)
		}
	}
}

// newSnapshot copies the games so a state refresh never mutates a snapshot
// that handlers are reading.
func (ix *Index) newSnapshot(games []*Game, states map[string]map[string]StateFile) *Snapshot {
	out := make([]*Game, 0, len(games))
	for _, g := range games {
		c := *g
		applyStateFlags(&c, states)
		c.HasImage = c.Image != "" || c.Thumb != ""
		_, c.Scrapable = ssSystemIDs[strings.ToLower(c.System)]
		c.HasVideo = c.Video != ""
		out = append(out, &c)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if a != b {
			return a < b
		}
		return out[i].File < out[j].File
	})

	snap := &Snapshot{
		Games:    out,
		ByID:     make(map[string]*Game, len(out)),
		BySystem: make(map[string][]*Game),
		Built:    time.Now(),
	}
	for _, g := range out {
		snap.ByID[g.ID] = g
		snap.BySystem[g.System] = append(snap.BySystem[g.System], g)
	}
	for key, list := range snap.BySystem {
		sys := System{Key: key, Name: ix.cfg.systemName(key), Games: len(list)}
		for _, g := range list {
			if g.HasState {
				sys.WithState++
			}
		}
		snap.Systems = append(snap.Systems, sys)
	}
	sort.Slice(snap.Systems, func(i, j int) bool {
		return strings.ToLower(snap.Systems[i].Name) < strings.ToLower(snap.Systems[j].Name)
	})
	return snap
}

func countWithState(games []*Game) int {
	n := 0
	for _, g := range games {
		if g.HasState {
			n++
		}
	}
	return n
}

// --------------------------------------------------------------- media ------

func resolveGamelistPath(sysDir, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = filepath.FromSlash(p)
	if strings.HasPrefix(p, "~/") {
		p = expandPath(p)
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(sysDir, p)
	}
	p = filepath.Clean(p)
	rel, err := filepath.Rel(sysDir, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return ""
	}
	return filepath.ToSlash(rel)
}

func resolveMedia(sysDir, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return ""
	}
	p = filepath.FromSlash(p)
	if strings.HasPrefix(p, "~/") {
		p = expandPath(p)
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(sysDir, p)
	}
	p = filepath.Clean(p)
	if isFile(p) {
		return p
	}
	return ""
}

// MediaPath resolves artwork lazily, so the initial scan never stats tens of
// thousands of image files.
func (ix *Index) MediaPath(g *Game, kind string) string {
	switch kind {
	case "boxart":
		if g.Image != "" {
			return g.Image
		}
	case "thumb":
		if g.Thumb != "" {
			return g.Thumb
		}
		if g.Image != "" {
			return g.Image
		}
	case "marquee":
		if g.Marquee != "" {
			return g.Marquee
		}
	case "video":
		if g.Video != "" {
			return g.Video
		}
	}

	sysDir := filepath.Join(ix.cfg.RomsRoot, g.System)
	base := g.Base
	var cands []string
	switch kind {
	case "boxart", "thumb":
		for _, sub := range []string{"images", filepath.Join("media", "images"), "downloaded_images"} {
			for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp"} {
				cands = append(cands, filepath.Join(sysDir, sub, base+ext))
			}
		}
	case "marquee":
		for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp"} {
			cands = append(cands, filepath.Join(sysDir, "images", base+"-marquee"+ext))
		}
	case "video":
		for _, ext := range []string{".mp4", ".webm"} {
			cands = append(cands, filepath.Join(sysDir, "videos", base+ext))
		}
	}
	for _, c := range cands {
		if isFile(c) {
			return c
		}
	}
	return ""
}

// ------------------------------------------------------------ helpers ------

// onlyExtensions limits which files count as games in systems whose folder is
// full of data files. Ports keep each port's assets (.exe, .dll, .pvr, saves…)
// next to its launcher, and only the .sh launchers are games.
var onlyExtensions = map[string]map[string]bool{
	"ports": {".sh": true},
}

var nonRomExt = map[string]bool{
	".xml": true, ".txt": true, ".md": true, ".nfo": true, ".pdf": true,
	".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true,
	".bmp": true, ".mp4": true, ".avi": true, ".mkv": true, ".webm": true,
	".mp3": true, ".ogg": true, ".flac": true, ".wav": true,
	".srm": true, ".rtc": true, ".sav": true, ".db": true, ".sqlite": true,
	".tmp": true, ".log": true, ".bak": true, ".old": true,
}

func isNonRomFile(name string) bool {
	lower := strings.ToLower(name)
	if lower == "gamelist.xml" || strings.HasPrefix(lower, "gamelist.xml.") ||
		lower == ".ds_store" || lower == "thumbs.db" ||
		strings.Contains(lower, ".multirk-bak-") || strings.HasSuffix(lower, ".multirk-tmp") {
		return true
	}
	if nonRomExt[filepath.Ext(lower)] {
		return true
	}
	if _, _, ok := splitStateName(name); ok {
		return true
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
