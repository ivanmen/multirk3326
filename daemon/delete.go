package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// removeRom deletes romsRoot/system/rel and then each file in media that lives
// inside the system folder. It refuses anything that is not a plain file
// inside that system folder, and never touches gamelists or other games.
// It returns how many media files were removed.
func removeRom(romsRoot, system, rel string, media []string) (int, error) {
	sysDir := filepath.Join(romsRoot, system)
	src := filepath.Join(sysDir, filepath.FromSlash(rel))
	r, err := filepath.Rel(sysDir, src)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return 0, errors.New("ruta fuera de la carpeta del sistema")
	}
	st, err := os.Lstat(src)
	if err != nil {
		return 0, errors.New("el archivo ya no existe en la consola")
	}
	if !st.Mode().IsRegular() {
		return 0, errors.New("no es un archivo normal; no se elimina")
	}
	if isNonRomFile(filepath.Base(src)) {
		return 0, errors.New("ese archivo no es una ROM")
	}
	if err := os.Remove(src); err != nil {
		return 0, fmt.Errorf("no se pudo eliminar: %v", err)
	}
	removed := 0
	for _, m := range media {
		mr, err := filepath.Rel(sysDir, m)
		if err != nil || mr == ".." || strings.HasPrefix(mr, ".."+string(os.PathSeparator)) {
			continue
		}
		if ms, err := os.Lstat(m); err == nil && ms.Mode().IsRegular() {
			if os.Remove(m) == nil {
				removed++
			}
		}
	}
	return removed, nil
}

// ownMedia lists the existing media files of g (cover, thumbnail, marquee,
// video) that no other game uses, so shared images are never deleted.
func (s *TCPServer) ownMedia(g *Game) []string {
	snap := s.ix.Snapshot()
	if snap == nil {
		return nil
	}
	others := map[string]bool{}
	sameBase := false
	for _, o := range snap.Games {
		if o.ID == g.ID {
			continue
		}
		for _, p := range []string{o.Image, o.Thumb, o.Marquee, o.Video} {
			if p != "" {
				others[p] = true
			}
		}
		if o.System == g.System && strings.EqualFold(o.Base, g.Base) {
			sameBase = true
		}
	}
	own := map[string]bool{}
	for _, p := range []string{g.Image, g.Thumb, g.Marquee, g.Video} {
		if p != "" {
			own[p] = true
		}
	}
	seen := map[string]bool{}
	var out []string
	for _, kind := range []string{"boxart", "thumb", "marquee", "video"} {
		p := s.ix.MediaPath(g, kind)
		if p == "" || seen[p] || others[p] {
			continue
		}
		// found by folder convention (named after the file): another game with
		// the same name in this system could own it too.
		if sameBase && !own[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func (s *TCPServer) deleteGame(g *Game) (map[string]any, error) {
	if g.Missing {
		return nil, errors.New("el archivo ya no existe en la consola")
	}
	if !scrapeMu.TryLock() {
		return nil, errors.New("hay otra operación de gamelist en curso")
	}
	defer scrapeMu.Unlock()

	media := s.ownMedia(g)
	n, err := removeRom(s.cfg.RomsRoot, g.System, g.RelPath, media)
	if err != nil {
		return nil, err
	}
	warn := ""
	glPath := filepath.Join(s.cfg.RomsRoot, g.System, "gamelist.xml")
	if err := applyGamelistRemove(glPath, g.RelPath); err != nil {
		warn = "la ROM se eliminó, pero no se pudo quitar su entrada del gamelist: " + err.Error()
	}
	if err := s.ix.ScanSystem(g.System); err != nil {
		warn = strings.TrimSpace(warn + " refresco: " + err.Error())
	}
	return map[string]any{"ok": true, "media": n, "warning": warn}, nil
}
