package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MULTIRK/2: compact binary frames over a persistent TCP connection.
//
//	0  2  magic "MK"      (0x4d4b)
//	2  1  version (2)
//	3  1  opcode
//	4  4  request id (BE, echoed)
//	8  4  flags (BE): 1=gzip payload, 4=error
//	12 4  payload length (BE)
//	16 .. payload
//
// Metadata travels as compact JSON (gzip when it is worth it); file contents
// travel as raw chunk frames. No HTTP, no chunked encoding, no per-request
// header parsing on the console.
const (
	protoMagic   = 0x4d4b
	protoVersion = 2

	frameHeaderLen = 16
	maxRequestLen  = 1 << 20 // 1 MB of JSON is plenty for any request
	maxFrameLen    = 4 << 20
	chunkSize      = 256 << 10

	flagGzip  = 1
	flagMore  = 2
	flagError = 4

	opHello   = 1
	opPair    = 2
	opInfo    = 3
	opSystems = 4
	opGames   = 5
	opGame    = 6
	opGet     = 7
	opChunk   = 8
	opEnd     = 9
	opPut     = 10
	opReady   = 11
	opPutOK   = 12
	opMedia   = 13
	opRefresh = 14
	opPing    = 15
	opControl = 16
	opStatus  = 17
	opCatalog = 18
	opScrape  = 19
	opSetMeta = 20
	opDelete  = 21
	opAudit   = 22
	opLaunch  = 23
	opStop    = 24
	opError   = 255
)

type frame struct {
	op      byte
	id      uint32
	flags   uint32
	payload []byte
}

func readFrame(r *bufio.Reader, max int64) (frame, error) {
	var h [frameHeaderLen]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return frame{}, err
	}
	if binary.BigEndian.Uint16(h[0:2]) != protoMagic {
		return frame{}, errors.New("magic inválido")
	}
	if h[2] != protoVersion {
		return frame{}, fmt.Errorf("protocolo %d no soportado", h[2])
	}
	f := frame{
		op:    h[3],
		id:    binary.BigEndian.Uint32(h[4:8]),
		flags: binary.BigEndian.Uint32(h[8:12]),
	}
	n := int64(binary.BigEndian.Uint32(h[12:16]))
	if n > max {
		return frame{}, fmt.Errorf("frame de %d bytes demasiado grande", n)
	}
	f.payload = make([]byte, n)
	if _, err := io.ReadFull(r, f.payload); err != nil {
		return frame{}, err
	}
	if f.flags&flagGzip != 0 {
		zr, err := gzip.NewReader(bytes.NewReader(f.payload))
		if err != nil {
			return frame{}, err
		}
		defer zr.Close()
		out, err := io.ReadAll(io.LimitReader(zr, 8<<20))
		if err != nil {
			return frame{}, err
		}
		f.payload = out
		f.flags &^= flagGzip
	}
	return f, nil
}

func writeFrame(w io.Writer, op byte, id uint32, flags uint32, payload []byte) error {
	var h [frameHeaderLen]byte
	binary.BigEndian.PutUint16(h[0:2], protoMagic)
	h[2] = protoVersion
	h[3] = op
	binary.BigEndian.PutUint32(h[4:8], id)
	binary.BigEndian.PutUint32(h[8:12], flags)
	binary.BigEndian.PutUint32(h[12:16], uint32(len(payload)))
	if _, err := w.Write(h[:]); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := w.Write(payload)
	return err
}

func encodeJSON(v any) ([]byte, uint32) {
	b, err := json.Marshal(v)
	if err != nil {
		b = []byte(`{"error":"json"}`)
	}
	if len(b) > 8192 {
		var zb bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&zb, gzip.BestSpeed)
		if _, err := zw.Write(b); err == nil && zw.Close() == nil && zb.Len() < len(b) {
			return zb.Bytes(), flagGzip
		}
	}
	return b, 0
}

// --------------------------------------------------------------- server ----

type TCPServer struct {
	cfg      *Config
	ix       *Index
	rt       *Runtime
	ctl      *Controls
	launcher Launcher
	started  time.Time
	lastUsed atomic.Int64

	catalogMu    sync.Mutex
	catalogStamp string
	catalogGz    []byte
	catalogRaw   int
}

func NewTCPServer(cfg *Config, ix *Index, rt *Runtime) *TCPServer {
	return &TCPServer{cfg: cfg, ix: ix, rt: rt, ctl: NewControls(cfg), started: time.Now()}
}

func (s *TCPServer) Serve(ctx context.Context, ln net.Listener) error {
	go s.idleRefresh(ctx)
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return err
		}
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.SetNoDelay(true)
			_ = tc.SetKeepAlive(true)
			_ = tc.SetKeepAlivePeriod(30 * time.Second)
		}
		s.lastUsed.Store(time.Now().Unix())
		go s.handleConn(ctx, conn)
	}
}

// idleRefresh keeps the state index warm only while somebody is using the
// console. With no clients the process does zero disk I/O.
func (s *TCPServer) idleRefresh(ctx context.Context) {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if time.Now().Unix()-s.lastUsed.Load() < 180 {
				s.ix.RefreshStates()
			}
		}
	}
}

func (s *TCPServer) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReaderSize(conn, 64<<10)
	w := bufio.NewWriterSize(conn, 64<<10)

	authed := !s.cfg.Auth
	ip := hostOf(conn.RemoteAddr())

	for {
		if ctx.Err() != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Minute))
		f, err := readFrame(r, maxRequestLen)
		if err != nil {
			return
		}
		s.lastUsed.Store(time.Now().Unix())

		switch f.op {
		case opHello:
			authed = s.handleHello(w, f, authed)
		case opPair:
			authed = s.handlePair(w, f, ip, authed)
		case opPing:
			s.sendJSON(w, f.id, opPing, map[string]any{"ts": time.Now().Unix(), "uptime": int64(time.Since(s.started).Seconds())})
		default:
			if !authed {
				s.sendError(w, f.id, 401, "token requerido")
				continue
			}
			if err := s.dispatch(w, r, conn, f); err != nil {
				return
			}
		}
	}
}

func (s *TCPServer) handleHello(w *bufio.Writer, f frame, authed bool) bool {
	var req struct {
		Token  string `json:"token"`
		Client string `json:"client"`
	}
	_ = json.Unmarshal(f.payload, &req)
	if s.cfg.Auth && s.rt.Authorized("Bearer "+req.Token) {
		authed = true
	}
	resp := map[string]any{
		"ok":       authed,
		"deviceId": s.rt.DeviceID,
		"name":     s.cfg.Name,
		"version":  version,
		"proto":    protoVersion,
		"auth":     s.cfg.Auth,
		"pair":     true,
		"maxChunk": chunkSize,
		"ts":       time.Now().Unix(),
	}
	if !authed {
		resp["error"] = "token requerido"
	}
	s.sendJSON(w, f.id, opHello, resp)
	return authed
}

func (s *TCPServer) handlePair(w *bufio.Writer, f frame, ip string, authed bool) bool {
	if !s.rt.allowAttempt(ip) {
		s.sendError(w, f.id, 429, "demasiados intentos, espera un minuto")
		return authed
	}
	var req struct {
		Client string `json:"client"`
	}
	if err := json.Unmarshal(f.payload, &req); err != nil {
		s.sendError(w, f.id, 400, "cuerpo inválido")
		return authed
	}
	c := s.rt.AddClient(req.Client)
	s.sendJSON(w, f.id, opPair, map[string]any{
		"ok":         true,
		"token":      c.Token,
		"deviceId":   s.rt.DeviceID,
		"deviceName": s.cfg.Name,
		"version":    version,
	})
	return true
}

func (s *TCPServer) dispatch(w *bufio.Writer, r *bufio.Reader, conn net.Conn, f frame) error {
	switch f.op {
	case opInfo:
		s.sendJSON(w, f.id, opInfo, buildInfo(s.cfg, s.ix, s.rt, s.started))

	case opSystems:
		snap := s.ix.Snapshot()
		systems := []System{}
		if snap != nil {
			systems = snap.Systems
		}
		s.sendJSON(w, f.id, opSystems, map[string]any{"systems": systems})

	case opGames:
		var req gamesReq
		if err := json.Unmarshal(f.payload, &req); err != nil {
			s.sendError(w, f.id, 400, "petición inválida")
			return nil
		}
		s.sendJSON(w, f.id, opGames, s.games(req))

	case opGame:
		g, artifacts, err := s.lookup(f.payload)
		if err != nil {
			s.sendError(w, f.id, 404, err.Error())
			return nil
		}
		s.sendJSON(w, f.id, opGame, map[string]any{"game": g, "artifacts": artifacts})

	case opGet, opMedia:
		if err := s.sendFile(w, f, conn); err != nil {
			s.sendError(w, f.id, 404, err.Error())
		}

	case opPut:
		if err := s.receiveFile(w, r, conn, f); err != nil {
			s.sendError(w, f.id, 400, err.Error())
		}

	case opScrape:
		var req struct {
			ID        string  `json:"id"`
			Lang      string  `json:"lang"`
			SS        ssCreds `json:"ss"`
			KeepImage bool    `json:"keepImage"`
		}
		if err := json.Unmarshal(f.payload, &req); err != nil {
			s.sendError(w, f.id, 400, "petición inválida")
			return nil
		}
		snap := s.ix.Snapshot()
		var g *Game
		if snap != nil {
			g = snap.ByID[req.ID]
		}
		if g == nil {
			s.sendError(w, f.id, 404, "juego no encontrado")
			return nil
		}
		res, err := s.scrapeGame(g, req.SS, firstNonEmpty(req.Lang, "es"), req.KeepImage)
		if err != nil {
			s.sendError(w, f.id, 502, err.Error())
			return nil
		}
		s.sendJSON(w, f.id, opScrape, res)

	case opSetMeta:
		var req struct {
			ID   string    `json:"id"`
			Meta metaInput `json:"meta"`
		}
		if err := json.Unmarshal(f.payload, &req); err != nil {
			s.sendError(w, f.id, 400, "petición inválida")
			return nil
		}
		snap := s.ix.Snapshot()
		var g *Game
		if snap != nil {
			g = snap.ByID[req.ID]
		}
		if g == nil {
			s.sendError(w, f.id, 404, "juego no encontrado")
			return nil
		}
		res, err := s.setGameMeta(g, req.Meta)
		if err != nil {
			s.sendError(w, f.id, 400, err.Error())
			return nil
		}
		s.sendJSON(w, f.id, opSetMeta, res)

	case opDelete:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(f.payload, &req); err != nil {
			s.sendError(w, f.id, 400, "petición inválida")
			return nil
		}
		snap := s.ix.Snapshot()
		var g *Game
		if snap != nil {
			g = snap.ByID[req.ID]
		}
		if g == nil {
			s.sendError(w, f.id, 404, "juego no encontrado")
			return nil
		}
		res, err := s.deleteGame(g)
		if err != nil {
			s.sendError(w, f.id, 400, err.Error())
			return nil
		}
		s.sendJSON(w, f.id, opDelete, res)

	case opLaunch:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(f.payload, &req); err != nil {
			s.sendError(w, f.id, 400, "petición inválida")
			return nil
		}
		snap := s.ix.Snapshot()
		var g *Game
		if snap != nil {
			g = snap.ByID[req.ID]
		}
		if g == nil {
			s.sendError(w, f.id, 404, "juego no encontrado")
			return nil
		}
		if err := s.launcher.Launch(s.cfg.RomsRoot, g); err != nil {
			s.sendError(w, f.id, 400, err.Error())
			return nil
		}
		s.sendJSON(w, f.id, opLaunch, s.launcher.State())

	case opStop:
		if err := s.launcher.Close(); err != nil {
			s.sendError(w, f.id, 400, err.Error())
			return nil
		}
		s.sendJSON(w, f.id, opStop, map[string]bool{"ok": true})

	case opAudit:
		var req struct {
			Dry bool `json:"dry"`
		}
		_ = json.Unmarshal(f.payload, &req)
		res, err := s.auditGamelists(req.Dry)
		if err != nil {
			s.sendError(w, f.id, 500, err.Error())
			return nil
		}
		s.sendJSON(w, f.id, opAudit, res)

	case opRefresh:
		var req struct {
			Wait   bool   `json:"wait"`
			System string `json:"system"`
		}
		_ = json.Unmarshal(f.payload, &req)
		scan := s.ix.Scan
		if req.System != "" {
			sys := req.System
			scan = func() error { return s.ix.ScanSystem(sys) }
		}
		if req.Wait {
			if err := scan(); err != nil {
				s.sendError(w, f.id, 500, err.Error())
				return nil
			}
		} else {
			go scan()
		}
		s.sendJSON(w, f.id, opRefresh, map[string]any{"ok": true, "scanning": !req.Wait, "system": req.System})

	case opStatus:
		s.sendJSON(w, f.id, opStatus, s.ctl.State())

	case opCatalog:
		if err := s.sendCatalog(w, f); err != nil {
			s.sendError(w, f.id, 500, err.Error())
		}

	case opControl:
		if !s.cfg.AllowControl {
			s.sendError(w, f.id, 403, "el control remoto está desactivado en la consola")
			return nil
		}
		var req struct {
			Action string `json:"action"`
			Value  int    `json:"value"`
			Mute   *bool  `json:"mute"`
		}
		if err := json.Unmarshal(f.payload, &req); err != nil {
			s.sendError(w, f.id, 400, "petición inválida")
			return nil
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
			s.sendError(w, f.id, 400, "acción inválida (volume|brightness|mute)")
			return nil
		}
		if err != nil {
			s.sendError(w, f.id, 500, err.Error())
			return nil
		}
		s.sendJSON(w, f.id, opControl, st)

	default:
		s.sendError(w, f.id, 404, fmt.Sprintf("opcode %d desconocido", f.op))
	}
	return nil
}

// --------------------------------------------------------------- catalog ---

type gamesReq struct {
	System         string `json:"system"`
	Q              string `json:"q"`
	MatchKey       string `json:"matchKey"`
	HasState       bool   `json:"hasState"`
	HasSram        bool   `json:"hasSram"`
	IncludeMissing bool   `json:"missing"`
	Offset         int    `json:"offset"`
	Limit          int    `json:"limit"`
	Sort           string `json:"sort"`
	Order          string `json:"order"`
}

func (s *TCPServer) games(req gamesReq) map[string]any {
	snap := s.ix.Snapshot()
	if snap == nil {
		return map[string]any{"total": 0, "games": []*Game{}}
	}
	system := strings.ToLower(strings.TrimSpace(req.System))
	query := strings.ToLower(strings.TrimSpace(req.Q))
	matchKey := strings.ToLower(strings.TrimSpace(req.MatchKey))
	limit := req.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	filtered := make([]*Game, 0, len(snap.Games))
	for _, g := range snap.Games {
		if system != "" && g.System != system {
			continue
		}
		if matchKey != "" && g.MatchKey != matchKey {
			continue
		}
		if req.HasState && !g.HasState {
			continue
		}
		if req.HasSram && !g.HasSram {
			continue
		}
		if !req.IncludeMissing && g.Missing {
			continue
		}
		if query != "" &&
			!strings.Contains(strings.ToLower(g.Name), query) &&
			!strings.Contains(strings.ToLower(g.File), query) {
			continue
		}
		filtered = append(filtered, g)
	}

	switch req.Sort {
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
	if req.Order == "desc" && (req.Sort == "" || req.Sort == "name") {
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
	return map[string]any{"total": total, "offset": offset, "limit": limit, "games": page}
}

func (s *TCPServer) lookup(payload []byte) (*Game, []Artifact, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, nil, errors.New("petición inválida")
	}
	snap := s.ix.Snapshot()
	if snap == nil {
		return nil, nil, errors.New("catálogo no disponible")
	}
	g := snap.ByID[req.ID]
	if g == nil {
		return nil, nil, errors.New("juego no encontrado")
	}
	artifacts := s.ix.Artifacts(g)
	if artifacts == nil {
		artifacts = []Artifact{}
	}
	return g, artifacts, nil
}

// ------------------------------------------------------------------ files ---

func (s *TCPServer) sendFile(w *bufio.Writer, f frame, conn net.Conn) error {
	var req struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
		Slot string `json:"slot"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(f.payload, &req); err != nil {
		return errors.New("petición inválida")
	}
	snap := s.ix.Snapshot()
	if snap == nil {
		return errors.New("catálogo no disponible")
	}
	g := snap.ByID[req.ID]
	if g == nil {
		return errors.New("juego no encontrado")
	}

	var path, name, kind, slot string
	if f.op == opMedia {
		typ := firstNonEmpty(req.Type, "boxart")
		path = s.ix.MediaPath(g, typ)
		if path == "" {
			return errors.New("imagen no encontrada")
		}
		name = filepath.Base(path)
		kind = "media"
		slot = typ
	} else {
		kind = firstNonEmpty(req.Kind, "state")
		slot = firstNonEmpty(req.Slot, "auto")
		sf, ok := s.ix.ResolveArtifact(g, kind, slot)
		if !ok {
			return errors.New("estado no encontrado")
		}
		path = sf.Path
		name = filepath.Base(sf.Path)
	}

	var data []byte
	if f.op == opMedia && (slot == "boxart" || slot == "thumb") {
		if b, err := scaledJPEG(path, 480); err == nil {
			data = b
			name = strings.TrimSuffix(name, filepath.Ext(name)) + ".jpg"
		}
	}

	var fh *os.File
	var size, mtime int64
	if data != nil {
		size, mtime = int64(len(data)), time.Now().Unix()
	} else {
		var err error
		fh, err = os.Open(path)
		if err != nil {
			return errors.New("no se pudo abrir el archivo")
		}
		defer fh.Close()
		st, err := fh.Stat()
		if err != nil {
			return err
		}
		size, mtime = st.Size(), st.ModTime().Unix()
	}

	meta := map[string]any{
		"size":  size,
		"mtime": mtime,
		"name":  name,
		"etag":  fileETag(size, mtime),
		"kind":  kind,
		"slot":  slot,
	}
	if ct := mimeByExt(name); ct != "" {
		meta["contentType"] = ct
	}
	if err := s.sendJSON(w, f.id, f.op, meta); err != nil {
		return err
	}

	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Minute))
	if data != nil {
		for off := 0; off < len(data); off += chunkSize {
			end := off + chunkSize
			if end > len(data) {
				end = len(data)
			}
			if err := writeFrame(w, opChunk, f.id, 0, data[off:end]); err != nil {
				return err
			}
		}
		if err := writeFrame(w, opEnd, f.id, 0, []byte(`{"ok":true}`)); err != nil {
			return err
		}
		return w.Flush()
	}
	buf := make([]byte, chunkSize)
	for {
		n, rerr := fh.Read(buf)
		if n > 0 {
			if err := writeFrame(w, opChunk, f.id, 0, buf[:n]); err != nil {
				return err
			}
			if err := w.Flush(); err != nil {
				return err
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	if err := writeFrame(w, opEnd, f.id, 0, []byte(`{"ok":true}`)); err != nil {
		return err
	}
	return w.Flush()
}

func (s *TCPServer) receiveFile(w *bufio.Writer, r *bufio.Reader, conn net.Conn, f frame) error {
	var req struct {
		ID      string `json:"id"`
		Kind    string `json:"kind"`
		Slot    string `json:"slot"`
		Size    int64  `json:"size"`
		Sha256  string `json:"sha256"`
		Mtime   int64  `json:"mtime"`
		Force   bool   `json:"force"`
		IfMatch string `json:"ifMatch"`
	}
	if err := json.Unmarshal(f.payload, &req); err != nil {
		return errors.New("petición inválida")
	}
	snap := s.ix.Snapshot()
	if snap == nil {
		return errors.New("catálogo no disponible")
	}
	g := snap.ByID[req.ID]
	if g == nil {
		return errors.New("juego no encontrado")
	}
	kind := firstNonEmpty(req.Kind, "state")
	slot := firstNonEmpty(req.Slot, "auto")

	maxBytes := int64(s.cfg.MaxUploadMB) * 1024 * 1024
	if req.Size < 0 || req.Size > maxBytes {
		return fmt.Errorf("tamaño inválido (máx %d MB)", s.cfg.MaxUploadMB)
	}
	target, err := s.ix.TargetPath(g, kind, slot)
	if err != nil {
		return err
	}

	current, hasCurrent := s.ix.ResolveArtifact(g, kind, slot)
	if !req.Force {
		switch {
		case req.IfMatch == "*" && hasCurrent:
			return errors.New("el estado ya existe")
		case req.IfMatch != "" && !hasCurrent:
			return errors.New("el estado ya no existe")
		case req.IfMatch != "" && req.IfMatch != fileETag(current.Size, current.Mtime):
			return errors.New("el estado cambió: descárgalo de nuevo antes de subir")
		}
	}

	spool := s.cfg.SpoolDir
	if spool == "" {
		spool = os.TempDir()
	}
	tmp, err := os.CreateTemp(spool, "multirk-put-*")
	if err != nil {
		return errors.New("no se pudo crear el archivo temporal")
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := s.sendJSON(w, f.id, opReady, map[string]any{"maxChunk": chunkSize, "ok": true}); err != nil {
		tmp.Close()
		return err
	}

	h := sha256.New()
	var received int64
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	for {
		cf, err := readFrame(r, chunkSize+4096)
		if err != nil {
			tmp.Close()
			return err
		}
		switch cf.op {
		case opChunk:
			received += int64(len(cf.payload))
			if received > req.Size {
				tmp.Close()
				return errors.New("se recibieron más bytes de los declarados")
			}
			if _, err := tmp.Write(cf.payload); err != nil {
				tmp.Close()
				return errors.New("fallo al escribir en el spool")
			}
			h.Write(cf.payload)
		case opEnd:
			goto done
		case opError:
			tmp.Close()
			return errors.New("el cliente canceló la subida")
		default:
			tmp.Close()
			return fmt.Errorf("frame inesperado %d", cf.op)
		}
	}
done:
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return errors.New("fallo de fsync en el spool")
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if received != req.Size {
		return fmt.Errorf("tamaño incompleto: %d de %d", received, req.Size)
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if req.Sha256 != "" && !strings.EqualFold(req.Sha256, sum) {
		return errors.New("checksum no coincide")
	}
	if _, err := commitTemp(tmpName, target, req.Mtime, s.cfg.Backups); err != nil {
		return errors.New("no se pudo guardar en la SD")
	}
	go s.ix.RefreshStates()

	outMtime := req.Mtime
	if st, err := os.Stat(target); err == nil {
		outMtime = st.ModTime().Unix()
	}
	err = s.sendJSON(w, f.id, opPutOK, map[string]any{
		"ok":     true,
		"size":   received,
		"sha256": sum,
		"mtime":  outMtime,
		"etag":   fileETag(received, outMtime),
		"file":   filepath.Base(target),
		"backup": s.cfg.Backups && hasCurrent,
	})
	if err == nil {
		log.Printf("put: %s/%s slot=%s %d bytes", g.System, g.Name, slot, received)
	}
	return err
}

func (s *TCPServer) sendJSON(w *bufio.Writer, id uint32, op byte, v any) error {
	b, flags := encodeJSON(v)
	if err := writeFrame(w, op, id, flags, b); err != nil {
		return err
	}
	return w.Flush()
}

func (s *TCPServer) sendError(w *bufio.Writer, id uint32, status int, msg string) {
	_ = s.sendJSON(w, id, opError, map[string]any{"error": msg, "status": status})
}

func hostOf(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	if host, _, err := net.SplitHostPort(addr.String()); err == nil {
		return host
	}
	return addr.String()
}

// --------------------------------------------------------------- catalog ---

func catalogFingerprint(snap *Snapshot) string {
	var b strings.Builder
	for _, g := range snap.Games {
		fmt.Fprintf(&b, "%s:%d:%d:%t;", g.ID, g.Mtime, g.StateMtime, g.HasState)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])[:16]
}

// sendCatalog sends the whole catalogue as one gzip frame. The blob is built
// once per fingerprint and kept in RAM, so a second phone/refresh costs no
// extra CPU and nothing is written to the SD card.
func (s *TCPServer) sendCatalog(w *bufio.Writer, f frame) error {
	var req struct {
		Since  string `json:"since"`
		System string `json:"system"`
	}
	_ = json.Unmarshal(f.payload, &req)

	snap := s.ix.Snapshot()
	if snap == nil {
		return errors.New("catálogo no disponible")
	}
	stamp := catalogFingerprint(snap)
	indexed := snap.Built.UTC().Format(time.RFC3339)

	if req.System != "" {
		return s.sendSystemCatalog(w, f, snap, req.System, stamp, indexed)
	}

	if req.Since != "" && req.Since == stamp {
		return s.sendJSON(w, f.id, opCatalog, map[string]any{
			"notModified": true,
			"stamp":       stamp,
			"indexedAt":   indexed,
		})
	}

	gz, err := s.catalogBlob(snap, stamp, indexed)
	if err != nil {
		return err
	}
	if err := writeFrame(w, opCatalog, f.id, flagGzip, gz); err != nil {
		return err
	}
	return w.Flush()
}

// sendSystemCatalog sends the same gzip catalogue shape but for one system
// only (small, so it is not cached): the phone replaces just that system.
func (s *TCPServer) sendSystemCatalog(w *bufio.Writer, f frame, snap *Snapshot, key, stamp, indexed string) error {
	var systems []System
	for _, sy := range snap.Systems {
		if sy.Key == key {
			systems = append(systems, sy)
		}
	}
	games := snap.BySystem[key]
	if games == nil {
		games = []*Game{}
	}
	if systems == nil {
		systems = []System{}
	}
	raw, err := json.Marshal(map[string]any{
		"stamp": stamp, "indexedAt": indexed, "system": key,
		"systems": systems, "games": games,
	})
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return err
	}
	if _, err := zw.Write(raw); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := writeFrame(w, opCatalog, f.id, flagGzip, buf.Bytes()); err != nil {
		return err
	}
	return w.Flush()
}

func (s *TCPServer) catalogBlob(snap *Snapshot, stamp, indexed string) ([]byte, error) {
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()

	if s.catalogStamp == stamp && s.catalogGz != nil {
		return s.catalogGz, nil
	}

	payload := struct {
		Stamp     string   `json:"stamp"`
		IndexedAt string   `json:"indexedAt"`
		Systems   []System `json:"systems"`
		Games     []*Game  `json:"games"`
	}{
		Stamp:     stamp,
		IndexedAt: indexed,
		Systems:   snap.Systems,
		Games:     snap.Games,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}

	s.catalogStamp = stamp
	s.catalogGz = buf.Bytes()
	s.catalogRaw = len(raw)
	log.Printf("catalog: %d juegos · %s JSON → %s gzip",
		len(snap.Games), humanBytes(int64(len(raw))), humanBytes(int64(len(s.catalogGz))))
	return s.catalogGz, nil
}

func mimeByExt(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	}
	return ""
}
