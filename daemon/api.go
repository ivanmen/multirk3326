package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	cfg     *Config
	ix      *Index
	rt      *Runtime
	ctl     *Controls
	started time.Time
}

func NewServer(cfg *Config, ix *Index, rt *Runtime) *Server {
	return &Server{cfg: cfg, ix: ix, rt: rt, ctl: NewControls(cfg), started: time.Now()}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", s.handleHealth)
	mux.HandleFunc("GET /v1/info", s.handleInfo)
	mux.HandleFunc("POST /v1/pair", s.handlePair)
	mux.HandleFunc("GET /v1/systems", s.auth(s.handleSystems))
	mux.HandleFunc("GET /v1/games", s.auth(s.handleGames))
	mux.HandleFunc("GET /v1/games/{id}", s.auth(s.handleGame))
	mux.HandleFunc("GET /v1/games/{id}/states", s.auth(s.handleStates))
	mux.HandleFunc("GET /v1/files/{id}", s.auth(s.handleFileGet))
	mux.HandleFunc("PUT /v1/files/{id}", s.auth(s.handleFilePut))
	mux.HandleFunc("GET /v1/media/{id}", s.auth(s.handleMedia))
	mux.HandleFunc("POST /v1/refresh", s.auth(s.handleRefresh))
	mux.HandleFunc("GET /v1/status", s.auth(s.handleStatus))
	mux.HandleFunc("POST /v1/control", s.auth(s.handleControl))
	return mux
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.cfg.Auth || s.rt.Authorized(r.Header.Get("Authorization")) {
			next(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="multirk"`)
		writeErr(w, http.StatusUnauthorized, "token requerido")
	}
}

// ------------------------------------------------------------------ info ---

type infoResp struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	Model        string `json:"model"`
	OS           string `json:"os"`
	RomsRoot     string `json:"romsRoot"`
	Systems      int    `json:"systems"`
	Games        int    `json:"games"`
	WithState    int    `json:"withState"`
	StatesAt     string `json:"statesAt"`
	IndexedAt    string `json:"indexedAt"`
	Scanning     bool   `json:"scanning"`
	LastError    string `json:"lastError,omitempty"`
	Auth         bool   `json:"auth"`
	PairOpen     bool   `json:"pairOpen"`
	Clients      int    `json:"clients"`
	Multicast    string `json:"multicast"`
	Port         int    `json:"port"`
	UptimeSec    int64  `json:"uptimeSec"`
	Protocol     string `json:"protocol"`
	System       SysInfo `json:"system"`
	Capabilities []string `json:"capabilities"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"version":      version,
		"uptimeSec":    int64(time.Since(s.started).Seconds()),
		"pairRequired": s.cfg.Auth,
	})
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, buildInfo(s.cfg, s.ix, s.rt, s.started))
}

func buildInfo(cfg *Config, ix *Index, rt *Runtime, started time.Time) infoResp {
	sys := readSysInfo(cfg.RomsRoot)
	model := sys.CPUModel
	if model == "" {
		model = "rk3326"
	}
	resp := infoResp{
		ID:        rt.DeviceID,
		Name:      cfg.Name,
		Version:   version,
		Model:     model,
		OS:        "arkos",
		RomsRoot:  cfg.RomsRoot,
		Auth:      cfg.Auth,
		PairOpen:  true,
		Clients:   rt.ClientCount(),
		Multicast: cfg.Multicast.Group,
		Port:      portOf(cfg.Listen),
		UptimeSec: int64(time.Since(started).Seconds()),
		Scanning:  ix.Scanning(),
		LastError: ix.LastError(),
		Protocol:  cfg.Protocol,
		System:    sys,
		Capabilities: []string{
			"catalog", "states", "sram", "media", "multicast", "atomic-write", "backup", "tcp-frames", "control", "battery", "catalog-gzip", "catalog-cache", "sysinfo",
		},
	}
	if snap := ix.Snapshot(); snap != nil {
		resp.Systems = len(snap.Systems)
		resp.Games = len(snap.Games)
		resp.WithState = countWithState(snap.Games)
		resp.IndexedAt = snap.Built.UTC().Format(time.RFC3339)
	}
	if at := ix.StatesAt(); !at.IsZero() {
		resp.StatesAt = at.UTC().Format(time.RFC3339)
	}
	return resp
}

func (s *Server) handlePair(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.rt.allowAttempt(ip) {
		writeErr(w, http.StatusTooManyRequests, "demasiados intentos, espera un minuto")
		return
	}
	var req struct {
		Client string `json:"client"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}
	client := s.rt.AddClient(req.Client)
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      client.Token,
		"deviceId":   s.rt.DeviceID,
		"deviceName": s.cfg.Name,
		"version":    version,
	})
}

// ---------------------------------------------------------------- catalog ---

func (s *Server) handleSystems(w http.ResponseWriter, r *http.Request) {
	snap := s.ix.Snapshot()
	if snap == nil {
		writeJSON(w, http.StatusOK, map[string]any{"systems": []System{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"systems": snap.Systems})
}

func (s *Server) handleGames(w http.ResponseWriter, r *http.Request) {
	snap := s.ix.Snapshot()
	if snap == nil {
		writeJSON(w, http.StatusOK, map[string]any{"total": 0, "games": []*Game{}})
		return
	}
	q := r.URL.Query()
	system := strings.ToLower(strings.TrimSpace(q.Get("system")))
	query := strings.ToLower(strings.TrimSpace(q.Get("q")))
	matchKey := strings.ToLower(strings.TrimSpace(q.Get("matchKey")))
	hasState := truthy(q.Get("hasState"))
	hasSram := truthy(q.Get("hasSram"))
	includeMissing := q.Get("missing") == "1"
	sortBy := q.Get("sort")
	order := q.Get("order")
	offset := intParam(q.Get("offset"), 0)
	limit := intParam(q.Get("limit"), 100)
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	filtered := make([]*Game, 0, len(snap.Games))
	for _, g := range snap.Games {
		if system != "" && g.System != system {
			continue
		}
		if matchKey != "" && g.MatchKey != matchKey {
			continue
		}
		if hasState && !g.HasState {
			continue
		}
		if hasSram && !g.HasSram {
			continue
		}
		if !includeMissing && g.Missing {
			continue
		}
		if query != "" {
			if !strings.Contains(strings.ToLower(g.Name), query) &&
				!strings.Contains(strings.ToLower(g.File), query) {
				continue
			}
		}
		filtered = append(filtered, g)
	}

	switch sortBy {
	case "mtime":
		sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].Mtime > filtered[j].Mtime })
	case "state":
		sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].StateMtime > filtered[j].StateMtime })
	case "size":
		sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].Size > filtered[j].Size })
	default:
		sort.SliceStable(filtered, func(i, j int) bool {
			a, b := strings.ToLower(filtered[i].Name), strings.ToLower(filtered[j].Name)
			if a != b {
				return a < b
			}
			return filtered[i].File < filtered[j].File
		})
	}
	if order == "desc" && (sortBy == "" || sortBy == "name") {
		for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
			filtered[i], filtered[j] = filtered[j], filtered[i]
		}
	}

	total := len(filtered)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := filtered[offset:end]
	if page == nil {
		page = []*Game{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":  total,
		"offset": offset,
		"limit":  limit,
		"games":  page,
	})
}

func (s *Server) handleGame(w http.ResponseWriter, r *http.Request) {
	g := s.lookupGame(w, r)
	if g == nil {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"game":      g,
		"artifacts": s.ix.Artifacts(g),
	})
}

func (s *Server) handleStates(w http.ResponseWriter, r *http.Request) {
	g := s.lookupGame(w, r)
	if g == nil {
		return
	}
	artifacts := s.ix.Artifacts(g)
	if artifacts == nil {
		artifacts = []Artifact{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"game":      g,
		"artifacts": artifacts,
	})
}

func (s *Server) lookupGame(w http.ResponseWriter, r *http.Request) *Game {
	id := r.PathValue("id")
	snap := s.ix.Snapshot()
	if snap == nil {
		writeErr(w, http.StatusServiceUnavailable, "catálogo no disponible, reintenta en unos segundos")
		return nil
	}
	g := snap.ByID[id]
	if g == nil {
		writeErr(w, http.StatusNotFound, "juego no encontrado")
		return nil
	}
	return g
}

// ------------------------------------------------------------------ files ---

func (s *Server) handleFileGet(w http.ResponseWriter, r *http.Request) {
	g := s.lookupGame(w, r)
	if g == nil {
		return
	}
	kind := firstNonEmpty(r.URL.Query().Get("kind"), "state")
	slot := firstNonEmpty(r.URL.Query().Get("slot"), "auto")

	sf, ok := s.ix.ResolveArtifact(g, kind, slot)
	if !ok {
		writeErr(w, http.StatusNotFound, "estado no encontrado")
		return
	}
	f, err := os.Open(sf.Path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "no se pudo abrir el estado")
		return
	}
	defer f.Close()

	etag := fileETag(sf.Size, sf.Mtime)
	w.Header().Set("ETag", etag)
	w.Header().Set("X-Mtime", strconv.FormatInt(sf.Mtime, 10))
	w.Header().Set("X-Size", strconv.FormatInt(sf.Size, 10))
	w.Header().Set("X-Kind", kind)
	if kind != "sram" && kind != "rtc" {
		w.Header().Set("X-Slot", slot)
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(sf.Path)))
	http.ServeContent(w, r, filepath.Base(sf.Path), time.Unix(sf.Mtime, 0), f)
}

func (s *Server) handleFilePut(w http.ResponseWriter, r *http.Request) {
	g := s.lookupGame(w, r)
	if g == nil {
		return
	}
	kind := firstNonEmpty(r.URL.Query().Get("kind"), "state")
	slot := firstNonEmpty(r.URL.Query().Get("slot"), "auto")
	force := truthy(r.URL.Query().Get("force"))

	current, hasCurrent := s.ix.ResolveArtifact(g, kind, slot)
	ifMatch := strings.TrimSpace(r.Header.Get("If-Match"))
	if !force && ifMatch != "" {
		if ifMatch == "*" {
			if hasCurrent {
				writeErr(w, http.StatusPreconditionFailed, "el estado ya existe")
				return
			}
		} else if !hasCurrent || ifMatch != fileETag(current.Size, current.Mtime) {
			writeErr(w, http.StatusPreconditionFailed, "el estado cambió: descárgalo de nuevo antes de subir")
			return
		}
	}

	maxBytes := int64(s.cfg.MaxUploadMB) * 1024 * 1024
	if cl := r.ContentLength; cl > maxBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "archivo demasiado grande")
		return
	}
	sum := firstNonEmpty(r.Header.Get("X-Checksum-Sha256"), r.Header.Get("X-Sha256"))
	mtime := int64Param(r.Header.Get("X-Mtime"), 0)

	size, writtenSha, target, err := s.ix.WriteArtifact(g, kind, slot, r.Body, maxBytes, sum, mtime, s.cfg.Backups)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, os.ErrPermission) {
			status = http.StatusInternalServerError
		}
		writeErr(w, status, err.Error())
		return
	}
	st, _ := os.Stat(target)
	mtimeOut := mtime
	if st != nil {
		mtimeOut = st.ModTime().Unix()
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":       true,
		"kind":     kind,
		"slot":     slot,
		"size":     size,
		"sha256":   writtenSha,
		"mtime":    mtimeOut,
		"etag":     fileETag(size, mtimeOut),
		"file":     filepath.Base(target),
		"backup":   s.cfg.Backups && hasCurrent,
	})
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	g := s.lookupGame(w, r)
	if g == nil {
		return
	}
	kind := firstNonEmpty(r.URL.Query().Get("type"), "boxart")
	p := s.ix.MediaPath(g, kind)
	if p == "" {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(p)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(p))); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("ETag", fileETag(st.Size(), st.ModTime().Unix()))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, filepath.Base(p), st.ModTime(), f)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {	if truthy(r.URL.Query().Get("wait")) {
		if err := s.ix.Scan(); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		go func() {
			if err := s.ix.Scan(); err != nil {
				// The error is exposed through /v1/info.
				return
			}
		}()
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "scanning": !truthy(r.URL.Query().Get("wait"))})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.ctl.State())
}

func (s *Server) handleControl(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.AllowControl {
		writeErr(w, http.StatusForbidden, "control remoto desactivado")
		return
	}
	var req struct {
		Action string `json:"action"`
		Value  int    `json:"value"`
		Mute   *bool  `json:"mute"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}
	var (
		st  ControlState
		err error
	)
	switch req.Action {
	case "volume":
		st, err = s.ctl.SetVolume(req.Value)
	case "brightness":
		st, err = s.ctl.SetBrightness(req.Value)
	case "mute":
		mute := true
		if req.Mute != nil {
			mute = *req.Mute
		}
		st, err = s.ctl.SetMute(mute)
	default:
		writeErr(w, http.StatusBadRequest, "acción inválida (volume|brightness|mute)")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// ----------------------------------------------------------------- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg, "status": status})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func fileETag(size, mtime int64) string {
	return fmt.Sprintf(`"%d-%d"`, size, mtime)
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "si", "sí", "on":
		return true
	}
	return false
}

func intParam(v string, def int) int {
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func int64Param(v string, def int64) int64 {
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return def
	}
	return n
}

func portOf(listen string) int {
	_, port, err := net.SplitHostPort(listen)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(port)
	return n
}
