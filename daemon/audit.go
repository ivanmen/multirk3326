package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Audit: compares every gamelist.xml against the ROMs on the SD and drops the
// entries whose file does not exist. It never touches ROMs or media, and every
// gamelist is written with the same backup + verification as any other edit.

type auditSystem struct {
	System  string   `json:"system"`
	Checked int      `json:"checked"`
	Removed int      `json:"removed"`
	Entries []string `json:"entries"`
}

var reAnyGameBlock = regexp.MustCompile(`(?s)[ \t]*<game\b[^>]*>.*?</game>[ \t]*\n?`)

// resolveEntryPath turns a gamelist <path> into an absolute path on disk, or ""
// when it cannot be trusted (outside of the folder that owns the gamelist).
func resolveEntryPath(dir, raw string) string {
	p := strings.TrimSpace(xmlUnesc(raw))
	if p == "" {
		return ""
	}
	p = filepath.FromSlash(p)
	if strings.HasPrefix(p, "~/") {
		p = expandPath(p)
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	p = filepath.Clean(p)
	rel, err := filepath.Rel(dir, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return ""
	}
	return p
}

// missingEntries returns the raw <path> of every <game> whose file is gone.
func missingEntries(dir, src string) (checked int, missing []string) {
	for _, b := range reAnyGameBlock.FindAllString(src, -1) {
		pm := rePathTag.FindStringSubmatch(b)
		if pm == nil {
			continue
		}
		abs := resolveEntryPath(dir, pm[1])
		if abs == "" {
			continue
		}
		checked++
		if _, err := os.Lstat(abs); err != nil && os.IsNotExist(err) {
			missing = append(missing, strings.TrimSpace(pm[1]))
		}
	}
	return checked, missing
}

func removeGameBlocks(src string, rawPaths []string) (string, int) {
	set := map[string]bool{}
	for _, p := range rawPaths {
		set[normGamePath(p)] = true
	}
	n := 0
	out := reAnyGameBlock.ReplaceAllStringFunc(src, func(b string) string {
		pm := rePathTag.FindStringSubmatch(b)
		if pm != nil && set[normGamePath(pm[1])] {
			n++
			return ""
		}
		return b
	})
	return out, n
}

// auditGamelistFile cleans one gamelist.xml and returns (checked, removed names).
func auditGamelistFile(glPath string, dry bool) (int, []string, error) {
	b, err := os.ReadFile(glPath)
	if err != nil {
		return 0, nil, nil
	}
	dir := filepath.Dir(glPath)
	checked, missing := missingEntries(dir, string(b))
	if len(missing) == 0 {
		return checked, nil, nil
	}
	if dry {
		return checked, missing, nil
	}
	err = applyGamelistEdit(glPath, func(src string) (string, int, error) {
		out, n := removeGameBlocks(src, missing)
		return out, -n, nil
	})
	if err != nil {
		return checked, nil, err
	}
	return checked, missing, nil
}

func (s *TCPServer) auditGamelists(dry bool) (map[string]any, error) {
	if !scrapeMu.TryLock() {
		return nil, fmt.Errorf("hay otra operación de gamelist en curso")
	}
	defer scrapeMu.Unlock()

	ents, err := os.ReadDir(s.cfg.RomsRoot)
	if err != nil {
		return nil, err
	}
	var report []auditSystem
	totalChecked, totalRemoved := 0, 0
	var firstErr error
	for _, d := range ents {
		name := d.Name()
		if !d.IsDir() || strings.HasPrefix(name, ".") || s.cfg.isExcluded(name) ||
			strings.Contains(strings.ToLower(name), "backup") {
			continue
		}
		sysDir := filepath.Join(s.cfg.RomsRoot, name)
		files := []string{filepath.Join(sysDir, "gamelist.xml")}
		if subs, err := os.ReadDir(sysDir); err == nil {
			for _, sd := range subs {
				if sd.IsDir() && !strings.HasPrefix(sd.Name(), ".") && !s.cfg.isExcluded(sd.Name()) {
					files = append(files, filepath.Join(sysDir, sd.Name(), "gamelist.xml"))
				}
			}
		}
		as := auditSystem{System: name}
		for _, f := range files {
			c, removed, err := auditGamelistFile(f, dry)
			if err != nil && firstErr == nil {
				firstErr = fmt.Errorf("%s: %v", name, err)
			}
			as.Checked += c
			as.Removed += len(removed)
			as.Entries = append(as.Entries, removed...)
		}
		totalChecked += as.Checked
		totalRemoved += as.Removed
		if as.Removed > 0 {
			sort.Strings(as.Entries)
			if len(as.Entries) > 25 {
				as.Entries = as.Entries[:25]
			}
			report = append(report, as)
			if !dry {
				_ = s.ix.ScanSystem(name)
			}
		}
	}
	res := map[string]any{
		"ok": true, "dry": dry, "checked": totalChecked, "removed": totalRemoved, "systems": report,
	}
	if firstErr != nil {
		res["warning"] = firstErr.Error()
	}
	return res, nil
}
