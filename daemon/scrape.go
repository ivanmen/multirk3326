package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Scraping lives on the console so the SD card never leaves the device: the
// phone only sends the order (and the user's ScreenScraper account); the agent
// downloads metadata + cover, patches that single <game> in gamelist.xml with
// a backup, verifies the result and refreshes its index.

type ssCreds struct {
	User    string `json:"user"`
	Pass    string `json:"pass"`
	DevID   string `json:"devid"`
	DevPass string `json:"devpass"`
}

type scraped struct {
	Name        string
	Desc        string
	Developer   string
	Publisher   string
	Genre       string
	Players     string
	Releasedate string
	Rating      float64
	ImageURL    string
	ImageExt    string
}

var ssSystemIDs = map[string]int{
	"neogeo": 142, "cps1": 6, "cps2": 7, "cps3": 8, "mame": 75, "mame2003": 75,
	"arcade": 75, "fbneo": 75, "fba": 75, "dreamcast": 23, "psp": 61, "ngp": 25,
	"ngpc": 82, "nes": 3, "fds": 106, "snes": 4, "gb": 9, "gbc": 10, "gba": 12,
	"megadrive": 1, "genesis": 1, "mastersystem": 2, "gamegear": 21, "psx": 57,
	"n64": 14, "pcengine": 31, "pcenginecd": 114, "dos": 135, "atari2600": 26,
	"atari7800": 41, "lynx": 28, "wonderswan": 45, "wonderswancolor": 46,
	"segacd": 20, "sega32x": 19, "saturn": 22, "nds": 15, "sg1000": 109,
	"coleco": 183, "msx": 113, "c64": 66, "amiga": 64, "scummvm": 123,
}

var scrapeMu sync.Mutex

var (
	esLoginOnce       sync.Once
	esLoginID, esLoginPW string
)

var reESDevLogin = regexp.MustCompile(`devid=([A-Za-z0-9_.\-]+)&devpassword=([A-Za-z0-9_.\-]+)`)

// esDevLogin reads the ScreenScraper developer login that the console's own
// EmulationStation already ships with. It is read on the console at run time
// and kept only in memory: it is never written to disk, logs or the phone.
func esDevLogin() (string, string) {
	esLoginOnce.Do(func() {
		for _, p := range []string{
			"/usr/bin/emulationstation/emulationstation",
			"/usr/bin/emulationstation",
		} {
			b, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			if m := reESDevLogin.FindSubmatch(b); m != nil {
				esLoginID, esLoginPW = string(m[1]), string(m[2])
				return
			}
		}
	})
	return esLoginID, esLoginPW
}

var httpScrape = &http.Client{Timeout: 40 * time.Second}

func ssText(items []any, lang string) string {
	pick := func(want string) string {
		for _, it := range items {
			m, _ := it.(map[string]any)
			l, _ := m["langue"].(string)
			if l == "" {
				l, _ = m["region"].(string)
			}
			if l == want {
				t, _ := m["text"].(string)
				if strings.TrimSpace(t) != "" {
					return t
				}
			}
		}
		return ""
	}
	for _, w := range []string{lang, "en", "wor", "us", "eu", "jp"} {
		if t := pick(w); t != "" {
			return t
		}
	}
	for _, it := range items {
		m, _ := it.(map[string]any)
		if t, _ := m["text"].(string); strings.TrimSpace(t) != "" {
			return t
		}
	}
	return ""
}

func ssSimple(v any) string {
	m, _ := v.(map[string]any)
	t, _ := m["text"].(string)
	return strings.TrimSpace(t)
}

func fetchScreenScraper(c ssCreds, system, file, lang string) (*scraped, error) {
	sysID, ok := ssSystemIDs[strings.ToLower(system)]
	if !ok {
		return nil, fmt.Errorf("el sistema %q no está soportado por ScreenScraper", system)
	}
	if c.DevID == "" || c.DevPass == "" {
		id, pw := esDevLogin()
		if id == "" {
			return nil, errors.New("no hay acceso de desarrollador: ES no está instalado o no lo incluye")
		}
		c.DevID, c.DevPass = id, pw
	}
	q := url.Values{}
	q.Set("devid", c.DevID)
	q.Set("devpassword", c.DevPass)
	q.Set("softname", "multirk")
	q.Set("output", "json")
	if c.User != "" && c.Pass != "" {
		q.Set("ssid", c.User)
		q.Set("sspassword", c.Pass)
	}
	q.Set("systemeid", strconv.Itoa(sysID))
	q.Set("romtype", "rom")
	q.Set("romnom", file)
	resp, err := httpScrape.Get("https://api.screenscraper.fr/api2/jeuInfos.php?" + q.Encode())
	if err != nil {
		return nil, fmt.Errorf("sin conexión con ScreenScraper: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	switch resp.StatusCode {
	case 401, 403:
		return nil, errors.New("ScreenScraper rechazó las credenciales")
	case 404:
		return nil, errors.New("ScreenScraper no encontró este juego")
	case 429, 430, 431:
		return nil, errors.New("límite diario de ScreenScraper alcanzado")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ScreenScraper respondió %d", resp.StatusCode)
	}
	var top struct {
		Response struct {
			Jeu map[string]any `json:"jeu"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &top); err != nil || top.Response.Jeu == nil {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 120 {
			msg = msg[:120]
		}
		return nil, fmt.Errorf("ScreenScraper: %s", msg)
	}
	jeu := top.Response.Jeu
	out := &scraped{}
	list := func(k string) []any { v, _ := jeu[k].([]any); return v }

	out.Name = ssText(list("noms"), "wor")
	out.Desc = strings.TrimSpace(ssText(list("synopsis"), lang))
	out.Developer = ssSimple(jeu["developpeur"])
	out.Publisher = ssSimple(jeu["editeur"])
	out.Players = ssSimple(jeu["joueurs"])
	if n, err := strconv.ParseFloat(ssSimple(jeu["note"]), 64); err == nil && n > 0 {
		out.Rating = n / 20
	}
	if d := ssText(list("dates"), "wor"); d != "" {
		out.Releasedate = ssDate(d)
	}
	for _, g := range list("genres") {
		gm, _ := g.(map[string]any)
		noms, _ := gm["noms"].([]any)
		if t := ssText(noms, lang); t != "" {
			out.Genre = t
			break
		}
	}

	medias := list("medias")
	best := func(types ...string) (string, string) {
		for _, t := range types {
			var cand map[string]any
			for _, mv := range medias {
				m, _ := mv.(map[string]any)
				if mt, _ := m["type"].(string); mt != t {
					continue
				}
				r, _ := m["region"].(string)
				if cand == nil || r == "wor" || (r == "us" && cand["region"] != "wor") {
					cand = m
				}
			}
			if cand != nil {
				u, _ := cand["url"].(string)
				f, _ := cand["format"].(string)
				if u != "" {
					return u, strings.ToLower(f)
				}
			}
		}
		return "", ""
	}
	out.ImageURL, out.ImageExt = best("box-2D", "mixrbv2", "ss", "sstitle")
	if out.ImageExt != "png" && out.ImageExt != "jpg" {
		out.ImageExt = "png"
	}
	if out.Desc == "" && out.ImageURL == "" && out.Developer == "" {
		return nil, errors.New("ScreenScraper no tiene datos de este juego")
	}
	return out, nil
}

func ssDate(d string) string {
	d = strings.TrimSpace(d)
	digits := strings.ReplaceAll(strings.ReplaceAll(d, "-", ""), "/", "")
	switch {
	case len(digits) >= 8:
		return digits[:8] + "T000000"
	case len(digits) == 6:
		return digits + "01T000000"
	case len(digits) == 4:
		return digits + "0101T000000"
	}
	return ""
}

// ---------------------------------------------------------------- gamelist ---

var (
	reGameBlock = regexp.MustCompile(`(?s)<game\b[^>]*>.*?</game>`)
	rePathTag   = regexp.MustCompile(`(?s)<path>(.*?)</path>`)
	reImageTag  = regexp.MustCompile(`<image>([^<]*)</image>`)
)

func xmlEsc(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func xmlUnesc(s string) string {
	r := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&apos;", "'", "&#39;", "'")
	return r.Replace(s)
}

func normGamePath(p string) string {
	p = strings.TrimSpace(xmlUnesc(p))
	p = strings.TrimPrefix(filepath.ToSlash(p), "./")
	return strings.ToLower(p)
}

// imageConvention reads how this system's gamelist already names images so a
// new cover lands in the same folder and style as the rest.
func imageConvention(content string) (dir string, suffix string) {
	counts := map[string]int{}
	suffixes := 0
	total := 0
	for _, m := range reImageTag.FindAllStringSubmatch(content, -1) {
		v := strings.TrimSpace(xmlUnesc(m[1]))
		if v == "" || strings.HasPrefix(v, "http") {
			continue
		}
		counts[path.Dir(v)]++
		total++
		base := strings.TrimSuffix(path.Base(v), path.Ext(v))
		if strings.HasSuffix(base, "-image") {
			suffixes++
		}
	}
	dir = "./media/images"
	top := 0
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if counts[k] > top {
			top, dir = counts[k], k
		}
	}
	if total > 0 && suffixes*2 > total {
		suffix = "-image"
	}
	return dir, suffix
}

type gameFields struct {
	Name        string
	SetName     bool
	Desc        string
	Developer   string
	Publisher   string
	Genre       string
	Players     string
	Releasedate string
	Rating      string
	Image       string
}

func (f gameFields) ordered() [][2]string {
	out := [][2]string{}
	if f.SetName {
		out = append(out, [2]string{"name", f.Name})
	}
	return append(out, [][2]string{
		{"desc", f.Desc}, {"developer", f.Developer}, {"publisher", f.Publisher},
		{"genre", f.Genre}, {"players", f.Players}, {"releasedate", f.Releasedate},
		{"rating", f.Rating}, {"image", f.Image},
	}...)
}

func setTag(block, tag, value, indent string) string {
	re := regexp.MustCompile(`(?s)<` + tag + `>.*?</` + tag + `>|<` + tag + `\s*/>`)
	rep := "<" + tag + ">" + xmlEsc(value) + "</" + tag + ">"
	if re.MatchString(block) {
		return re.ReplaceAllLiteralString(block, rep)
	}
	i := strings.LastIndex(block, "</game>")
	if i < 0 {
		return block
	}
	head := strings.TrimRight(block[:i], " \t")
	if !strings.HasSuffix(head, "\n") {
		head += "\n"
	}
	return head + indent + rep + "\n" + indent[:max(0, len(indent)-1)] + block[i:]
}

// patchGamelistText returns the new gamelist text with only the target <game>
// changed (or appended). Every byte outside that block stays identical.
func patchGamelistText(src, relPath string, f gameFields) (string, bool, error) {
	target := normGamePath(relPath)
	found := false
	out := reGameBlock.ReplaceAllStringFunc(src, func(block string) string {
		if found {
			return block
		}
		pm := rePathTag.FindStringSubmatch(block)
		if pm == nil || normGamePath(pm[1]) != target {
			return block
		}
		found = true
		indent := "\t\t"
		if i := strings.Index(block, "<path>"); i > 0 {
			ls := strings.LastIndex(block[:i], "\n") + 1
			if ind := block[ls:i]; strings.TrimSpace(ind) == "" && ind != "" {
				indent = ind
			}
		}
		for _, kv := range f.ordered() {
			if kv[1] != "" {
				block = setTag(block, kv[0], kv[1], indent)
			}
		}
		return block
	})
	if found {
		return out, false, nil
	}
	i := strings.LastIndex(src, "</gameList>")
	if i < 0 {
		return "", false, errors.New("gamelist.xml sin </gameList>")
	}
	var b strings.Builder
	b.WriteString("\t<game>\n\t\t<path>./" + xmlEsc(relPath) + "</path>\n")
	if f.Name != "" {
		b.WriteString("\t\t<name>" + xmlEsc(f.Name) + "</name>\n")
	}
	for _, kv := range f.ordered() {
		if kv[1] != "" && kv[0] != "name" {
			b.WriteString("\t\t<" + kv[0] + ">" + xmlEsc(kv[1]) + "</" + kv[0] + ">\n")
		}
	}
	b.WriteString("\t</game>\n")
	head := strings.TrimRight(src[:i], " \t")
	if !strings.HasSuffix(head, "\n") {
		head += "\n"
	}
	return head + b.String() + src[i:], true, nil
}

func countGames(s string) (int, error) {
	dec := xml.NewDecoder(strings.NewReader(s))
	n := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return n, nil
		}
		if err != nil {
			return 0, err
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "game" {
			n++
		}
	}
}

func pruneBackups(glPath string) {
	matches, _ := filepath.Glob(glPath + ".multirk-bak-*")
	sort.Strings(matches)
	for len(matches) > 3 {
		_ = os.Remove(matches[0])
		matches = matches[1:]
	}
}

// applyGamelistPatch backs the file up, writes atomically and verifies; on any
// problem the original is restored.
func applyGamelistPatch(glPath, relPath string, f gameFields) error {
	return applyGamelistEdit(glPath, func(src string) (string, int, error) {
		out, added, err := patchGamelistText(src, relPath, f)
		delta := 0
		if added {
			delta = 1
		}
		return out, delta, err
	})
}

// removeGameBlock deletes the <game> whose <path> matches, leaving every other
// byte of the file untouched. removed is false when no entry matched.
func removeGameBlock(src, relPath string) (out string, removed bool) {
	target := normGamePath(relPath)
	re := regexp.MustCompile(`(?s)[ \t]*<game\b[^>]*>.*?</game>[ \t]*\n?`)
	out = re.ReplaceAllStringFunc(src, func(block string) string {
		if removed {
			return block
		}
		pm := rePathTag.FindStringSubmatch(block)
		if pm == nil || normGamePath(pm[1]) != target {
			return block
		}
		removed = true
		return ""
	})
	return out, removed
}

// applyGamelistRemove removes one game's entry with the same backup/verify
// safety as every other gamelist write. A missing entry is not an error.
func applyGamelistRemove(glPath, relPath string) error {
	if _, err := os.Stat(glPath); err != nil {
		return nil
	}
	return applyGamelistEdit(glPath, func(src string) (string, int, error) {
		out, removed := removeGameBlock(src, relPath)
		if !removed {
			return src, 0, nil
		}
		return out, -1, nil
	})
}

// applyGamelistEdit is the safe-write core: validate, back up, write via a
// temp file, re-read and compare; restore the original on any mismatch.
func applyGamelistEdit(glPath string, edit func(src string) (string, int, error)) error {
	orig, err := os.ReadFile(glPath)
	missing := false
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		orig = []byte("<?xml version=\"1.0\"?>\n<gameList>\n</gameList>\n")
		missing = true
	}
	src := string(orig)
	before, err := countGames(src)
	if err != nil {
		return fmt.Errorf("el gamelist actual no es XML válido: %v", err)
	}
	out, delta, err := edit(src)
	if err != nil {
		return err
	}
	if out == src {
		return nil
	}
	after, err := countGames(out)
	if err != nil {
		return fmt.Errorf("el resultado no es XML válido: %v", err)
	}
	if after != before+delta {
		return fmt.Errorf("verificación fallida: juegos %d→%d", before, after)
	}

	mode := os.FileMode(0o644)
	bak := ""
	if !missing {
		if st, err := os.Stat(glPath); err == nil {
			mode = st.Mode().Perm()
		}
		bak = fmt.Sprintf("%s.multirk-bak-%d", glPath, time.Now().Unix())
		if err := os.WriteFile(bak, orig, mode); err != nil {
			return fmt.Errorf("no se pudo crear el backup: %v", err)
		}
		if b, err := os.ReadFile(bak); err != nil || string(b) != src {
			return errors.New("el backup no coincide con el original")
		}
	}
	tmp := glPath + ".multirk-tmp"
	if err := os.WriteFile(tmp, []byte(out), mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, glPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if b, err := os.ReadFile(glPath); err != nil || string(b) != out {
		if bak != "" {
			_ = os.WriteFile(glPath, orig, mode)
		}
		return errors.New("la escritura no coincide; se restauró el original")
	}
	if bak != "" {
		pruneBackups(glPath)
	}
	return nil
}

func downloadImage(rawURL, dest string) error {
	resp, err := httpScrape.Get(rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("imagen: HTTP %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "image/") {
		return fmt.Errorf("imagen: tipo %s", ct)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".multirk-tmp"
	fh, err := os.Create(tmp)
	if err != nil {
		return err
	}
	n, err := io.Copy(fh, io.LimitReader(resp.Body, 12<<20))
	fh.Close()
	if err != nil || n == 0 {
		_ = os.Remove(tmp)
		return errors.New("imagen vacía o cortada")
	}
	return os.Rename(tmp, dest)
}

var reUnsafe = regexp.MustCompile(`[\\/:*?"<>|]`)

// scrapeGame runs the whole pipeline for one game and refreshes the index.
func (s *TCPServer) scrapeGame(g *Game, creds ssCreds, lang string, keepImage bool) (map[string]any, error) {
	if !scrapeMu.TryLock() {
		return nil, errors.New("ya hay un scraping en curso")
	}
	defer scrapeMu.Unlock()

	data, err := fetchScreenScraper(creds, g.System, g.File, lang)
	if err != nil {
		return nil, err
	}
	sysDir := filepath.Join(s.cfg.RomsRoot, g.System)
	glPath := filepath.Join(sysDir, "gamelist.xml")
	existing, _ := os.ReadFile(glPath)
	dir, suffix := imageConvention(string(existing))

	fields := gameFields{
		Name: data.Name, Desc: data.Desc, Developer: data.Developer,
		Publisher: data.Publisher, Genre: data.Genre, Players: data.Players,
		Releasedate: data.Releasedate,
	}
	if data.Rating > 0 {
		fields.Rating = strconv.FormatFloat(data.Rating, 'f', 2, 64)
	}
	imageSaved := ""
	if data.ImageURL != "" && !keepImage {
		base := reUnsafe.ReplaceAllString(g.Base, "_")
		rel := path.Join(dir, base+suffix+"."+data.ImageExt)
		dest := filepath.Join(sysDir, filepath.FromSlash(strings.TrimPrefix(rel, "./")))
		if err := downloadImage(data.ImageURL, dest); err == nil {
			fields.Image = "./" + strings.TrimPrefix(rel, "./")
			imageSaved = fields.Image
		}
	}
	if err := applyGamelistPatch(glPath, g.RelPath, fields); err != nil {
		return nil, err
	}
	if err := s.ix.ScanSystem(g.System); err != nil {
		return nil, fmt.Errorf("se guardó, pero falló el refresco: %v", err)
	}
	updated := []string{}
	for _, kv := range fields.ordered() {
		if kv[1] != "" {
			updated = append(updated, kv[0])
		}
	}
	return map[string]any{
		"ok": true, "source": "screenscraper", "updated": updated,
		"image": imageSaved, "lang": lang,
	}, nil
}
