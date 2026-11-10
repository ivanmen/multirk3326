package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// Manual metadata: for systems ScreenScraper does not cover (ports, ...), the
// phone sends the fields and a small picture; the agent writes them with the
// very same safe gamelist patch used by the scraper.

type metaInput struct {
	Name        string  `json:"name"`
	Desc        string  `json:"desc"`
	Developer   string  `json:"developer"`
	Publisher   string  `json:"publisher"`
	Genre       string  `json:"genre"`
	Players     string  `json:"players"`
	Releasedate string  `json:"releasedate"`
	Rating      float64 `json:"rating"`
	// Image is a base64 JPEG/PNG (the phone already shrinks it).
	Image string `json:"image"`
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		s = s[:n]
	}
	return s
}

func (s *TCPServer) setGameMeta(g *Game, in metaInput) (map[string]any, error) {
	if !scrapeMu.TryLock() {
		return nil, errors.New("hay otra operación de gamelist en curso")
	}
	defer scrapeMu.Unlock()

	sysDir := filepath.Join(s.cfg.RomsRoot, g.System)
	glPath := filepath.Join(sysDir, "gamelist.xml")
	existing, _ := os.ReadFile(glPath)

	fields := gameFields{
		Name: clip(in.Name, 200), SetName: strings.TrimSpace(in.Name) != "",
		Desc: clip(in.Desc, 6000), Developer: clip(in.Developer, 200),
		Publisher: clip(in.Publisher, 200), Genre: clip(in.Genre, 200),
		Players: clip(in.Players, 40), Releasedate: clip(in.Releasedate, 20),
	}
	if in.Rating > 0 {
		if in.Rating > 1 {
			in.Rating = 1
		}
		fields.Rating = strconv.FormatFloat(in.Rating, 'f', 2, 64)
	}

	imageSaved := ""
	if in.Image != "" {
		raw, err := base64.StdEncoding.DecodeString(in.Image)
		if err != nil || len(raw) == 0 || len(raw) > 3<<20 {
			return nil, errors.New("imagen inválida o demasiado grande")
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
		if err != nil || (format != "jpeg" && format != "png") || cfg.Width < 1 || cfg.Width > 4096 || cfg.Height > 4096 {
			return nil, errors.New("la imagen no es JPEG/PNG válida")
		}
		ext := "jpg"
		if format == "png" {
			ext = "png"
		}
		dir, suffix := imageConvention(string(existing))
		base := reUnsafe.ReplaceAllString(g.Base, "_")
		rel := path.Join(dir, base+suffix+"."+ext)
		dest := filepath.Join(sysDir, filepath.FromSlash(strings.TrimPrefix(rel, "./")))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, err
		}
		tmp := dest + ".multirk-tmp"
		if err := os.WriteFile(tmp, raw, 0o644); err != nil {
			return nil, err
		}
		if err := os.Rename(tmp, dest); err != nil {
			_ = os.Remove(tmp)
			return nil, err
		}
		fields.Image = "./" + strings.TrimPrefix(rel, "./")
		imageSaved = fields.Image
	}

	updated := []string{}
	for _, kv := range fields.ordered() {
		if kv[1] != "" {
			updated = append(updated, kv[0])
		}
	}
	if len(updated) == 0 {
		return nil, errors.New("no hay nada que guardar")
	}
	if err := applyGamelistPatch(glPath, g.RelPath, fields); err != nil {
		return nil, err
	}
	if err := s.ix.ScanSystem(g.System); err != nil {
		return nil, fmt.Errorf("se guardó, pero falló el refresco: %v", err)
	}
	return map[string]any{"ok": true, "source": "manual", "updated": updated, "image": imageSaved}, nil
}
