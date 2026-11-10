// multirkctl is the reference client for multirkd: it speaks the binary
// MULTIRK/2 protocol over TCP (fast path), discovers consoles over UDP
// multicast, and can install the agent on an ArkOS console over SSH.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var (
	addr    = flag.String("addr", envOr("MULTIRK_ADDR", "127.0.0.1:47820"), "host:puerto del daemon")
	token   = flag.String("token", os.Getenv("MULTIRK_TOKEN"), "token Bearer")
	clientName = flag.String("client", hostname(), "nombre de este cliente al emparejar")
	timeout = flag.Duration("timeout", 30*time.Second, "timeout de conexión")
)

func main() {
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	var err error
	switch args[0] {
	case "info":
		err = cmdInfo()
	case "systems":
		err = cmdSystems()
	case "games":
		err = cmdGames(args[1:])
	case "game":
		err = cmdGame(args[1:], true)
	case "states":
		err = cmdGame(args[1:], false)
	case "pull":
		err = cmdPull(args[1:])
	case "push":
		err = cmdPush(args[1:])
	case "media":
		err = cmdMedia(args[1:])
	case "refresh":
		err = cmdRefresh(args[1:])
	case "scrape":
		err = cmdScrape(args[1:])
	case "meta":
		err = cmdMeta(args[1:])
	case "delete":
		err = cmdDelete(args[1:])
	case "launch":
		err = cmdLaunch(args[1:], opLaunch)
	case "stop":
		err = cmdLaunch(args[1:], opStop)
	case "audit":
		err = cmdAudit(args[1:])
	case "status":
		err = cmdStatus()
	case "control":
		err = cmdControl(args[1:])
	case "ping":
		err = cmdPing()
	case "discover":
		err = cmdDiscover(args[1:])
	case "install":
		err = cmdInstall(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n", args[0])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `multirkctl — cliente de referencia para multirkd (MULTIRK/2 sobre TCP)

  multirkctl [-addr host:puerto] [-token T] <comando>

comandos:
  ping                       comprobar conexión
  info                       resumen del daemon
  systems                    sistemas y conteos
  games [-system s] [-q txt] [-state] [-limit n]
  game  -id ID               ficha + artefactos
  states -id ID              estados guardados
  pull  -id ID [-kind state|sram|rtc] [-slot auto] [-out archivo]
  push  -id ID -in archivo [-kind state|sram|rtc] [-slot auto] [-force]
  media -id ID [-type boxart] [-out archivo]
  refresh [-wait]
  status                     volumen, brillo y batería
  control -volume 40 -brightness 70 [-mute|-unmute]
  discover [-group 239.255.42.99] [-port 47820] [-timeout 5s] [-iface NOMBRE]
  install -host IP [-user ark] [-port 22] [-key id_ed25519 | -password ark] [-name RG351V]

El protocolo no usa HTTP: frames binarios de 16 bytes sobre TCP.
`)
}

// ---------------------------------------------------------------- client ----

type client struct {
	conn  net.Conn
	r     *bufio.Reader
	w     *bufio.Writer
	id    uint32
	token string
	info  map[string]any
}

func dial(addr string) (*client, error) {
	conn, err := net.DialTimeout("tcp", addr, *timeout)
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar a %s: %w", addr, err)
	}
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}
	c := &client{conn: conn, r: bufio.NewReaderSize(conn, 64<<10), w: bufio.NewWriterSize(conn, 64<<10)}

	resp, err := c.call(opHello, map[string]any{"token": *token, "client": *clientName, "proto": 2})
	if err != nil {
		conn.Close()
		return nil, err
	}
	var hello map[string]any
	_ = json.Unmarshal(resp.payload, &hello)
	c.info = hello
	if ok, _ := hello["ok"].(bool); !ok {
		{
			pr, err := c.call(opPair, map[string]any{"client": *clientName})
			if err != nil {
				conn.Close()
				return nil, err
			}
			var paired map[string]any
			_ = json.Unmarshal(pr.payload, &paired)
			tok, _ := paired["token"].(string)
			if tok == "" {
				conn.Close()
				return nil, errors.New("el emparejamiento no devolvió token")
			}
			c.token = tok
			fmt.Fprintf(os.Stderr, "emparejado · token: %s\n", tok)
			return c, nil
		}
	}
	if t, ok := hello["token"].(string); ok {
		c.token = t
	}
	return c, nil
}

func (c *client) Close() { c.conn.Close() }

func (c *client) call(op byte, req any) (frame, error) {
	c.id++
	var payload []byte
	if req != nil {
		b, err := json.Marshal(req)
		if err != nil {
			return frame{}, err
		}
		payload = b
	}
	_ = c.conn.SetDeadline(time.Now().Add(*timeout))
	if err := writeFrame(c.w, op, c.id, 0, payload); err != nil {
		return frame{}, err
	}
	if err := c.w.Flush(); err != nil {
		return frame{}, err
	}
	f, err := readFrame(c.r, 32<<20)
	if err != nil {
		return frame{}, err
	}
	if f.op == opError {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(f.payload, &e)
		if e.Error == "" {
			e.Error = "error del servidor"
		}
		return f, errors.New(e.Error)
	}
	return f, nil
}

func (c *client) getFile(op byte, req any, out io.Writer) (map[string]any, error) {
	c.id++
	payload, _ := json.Marshal(req)
	_ = c.conn.SetDeadline(time.Now().Add(10 * time.Minute))
	if err := writeFrame(c.w, op, c.id, 0, payload); err != nil {
		return nil, err
	}
	if err := c.w.Flush(); err != nil {
		return nil, err
	}
	metaFrame, err := readFrame(c.r, 1<<20)
	if err != nil {
		return nil, err
	}
	if metaFrame.op == opError {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(metaFrame.payload, &e)
		return nil, errors.New(e.Error)
	}
	var meta map[string]any
	_ = json.Unmarshal(metaFrame.payload, &meta)
	for {
		f, err := readFrame(c.r, maxFrameLen)
		if err != nil {
			return meta, err
		}
		switch f.op {
		case opChunk:
			if _, err := out.Write(f.payload); err != nil {
				return meta, err
			}
		case opEnd:
			return meta, nil
		case opError:
			var e struct {
				Error string `json:"error"`
			}
			_ = json.Unmarshal(f.payload, &e)
			return meta, errors.New(e.Error)
		}
	}
}

func (c *client) putFile(req map[string]any, data []byte) (map[string]any, error) {
	c.id++
	payload, _ := json.Marshal(req)
	_ = c.conn.SetDeadline(time.Now().Add(10 * time.Minute))
	if err := writeFrame(c.w, opPut, c.id, 0, payload); err != nil {
		return nil, err
	}
	if err := c.w.Flush(); err != nil {
		return nil, err
	}
	ready, err := readFrame(c.r, 1<<20)
	if err != nil {
		return nil, err
	}
	if ready.op == opError {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(ready.payload, &e)
		return nil, errors.New(e.Error)
	}
	for off := 0; off < len(data); off += chunkSize {
		end := off + chunkSize
		if end > len(data) {
			end = len(data)
		}
		if err := writeFrame(c.w, opChunk, c.id, 0, data[off:end]); err != nil {
			return nil, err
		}
	}
	if err := writeFrame(c.w, opEnd, c.id, 0, nil); err != nil {
		return nil, err
	}
	if err := c.w.Flush(); err != nil {
		return nil, err
	}
	resp, err := readFrame(c.r, 1<<20)
	if err != nil {
		return nil, err
	}
	if resp.op == opError {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(resp.payload, &e)
		return nil, errors.New(e.Error)
	}
	var out map[string]any
	_ = json.Unmarshal(resp.payload, &out)
	return out, nil
}

// -------------------------------------------------------------- framing ----

const (
	protoMagic   = 0x4d4b
	protoVersion = 2

	frameHeaderLen = 16
	maxFrameLen    = 4 << 20
	chunkSize      = 256 << 10

	flagGzip = 1

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
	opScrape  = 19
	opSetMeta = 20
	opDelete  = 21
	opAudit   = 22
	opLaunch  = 23
	opStop    = 24
	opPing    = 15
	opControl = 16
	opStatus  = 17
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
		return frame{}, errors.New("respuesta no es MULTIRK/2")
	}
	if h[2] != protoVersion {
		return frame{}, fmt.Errorf("versión de protocolo %d no soportada", h[2])
	}
	f := frame{op: h[3], id: binary.BigEndian.Uint32(h[4:8]), flags: binary.BigEndian.Uint32(h[8:12])}
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
		out, err := io.ReadAll(io.LimitReader(zr, 32<<20))
		if err != nil {
			return frame{}, err
		}
		f.payload = out
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

// -------------------------------------------------------------- commands ----

func cmdPing() error {
	c, err := dial(*addr)
	if err != nil {
		return err
	}
	defer c.Close()
	f, err := c.call(opPing, nil)
	if err != nil {
		return err
	}
	var out map[string]any
	_ = json.Unmarshal(f.payload, &out)
	fmt.Printf("ok · uptime %vs · proto %v\n", out["uptime"], c.info["proto"])
	return nil
}

func cmdInfo() error {
	c, err := dial(*addr)
	if err != nil {
		return err
	}
	defer c.Close()
	f, err := c.call(opInfo, nil)
	if err != nil {
		return err
	}
	return printJSON(f.payload)
}

func cmdSystems() error {
	c, err := dial(*addr)
	if err != nil {
		return err
	}
	defer c.Close()
	f, err := c.call(opSystems, nil)
	if err != nil {
		return err
	}
	var resp struct {
		Systems []struct {
			Key       string `json:"key"`
			Name      string `json:"name"`
			Games     int    `json:"games"`
			WithState int    `json:"withState"`
		} `json:"systems"`
	}
	if err := json.Unmarshal(f.payload, &resp); err != nil {
		return err
	}
	for _, s := range resp.Systems {
		fmt.Printf("%-14s %-26s %5d juegos  %4d con estado\n", s.Key, s.Name, s.Games, s.WithState)
	}
	return nil
}

func cmdGames(args []string) error {
	fs := flag.NewFlagSet("games", flag.ExitOnError)
	system := fs.String("system", "", "filtrar sistema")
	q := fs.String("q", "", "buscar texto")
	hasState := fs.Bool("state", false, "solo con estado")
	matchKey := fs.String("match", "", "matchKey entre consolas")
	limit := fs.Int("limit", 50, "máximo de resultados")
	fs.Parse(args)

	c, err := dial(*addr)
	if err != nil {
		return err
	}
	defer c.Close()
	f, err := c.call(opGames, map[string]any{
		"system": *system, "q": *q, "hasState": *hasState,
		"matchKey": *matchKey, "limit": *limit,
	})
	if err != nil {
		return err
	}
	var resp struct {
		Total int `json:"total"`
		Games []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			System     string `json:"system"`
			File       string `json:"file"`
			Size       int64  `json:"size"`
			HasState   bool   `json:"hasState"`
			StateMtime int64  `json:"stateMtime"`
		} `json:"games"`
	}
	if err := json.Unmarshal(f.payload, &resp); err != nil {
		return err
	}
	fmt.Printf("%d resultado(s), mostrando %d\n", resp.Total, len(resp.Games))
	for _, g := range resp.Games {
		mark := " "
		if g.HasState {
			mark = "●"
		}
		fmt.Printf("%s %-16s %-40s %-10s %8s  %s\n", mark, g.ID, truncate(g.Name, 40), g.System, humanBytes(g.Size), g.File)
	}
	return nil
}

func cmdGame(args []string, full bool) error {
	fs := flag.NewFlagSet("game", flag.ExitOnError)
	id := fs.String("id", "", "id del juego")
	fs.Parse(args)
	if *id == "" {
		return errors.New("falta -id")
	}
	c, err := dial(*addr)
	if err != nil {
		return err
	}
	defer c.Close()
	f, err := c.call(opGame, map[string]any{"id": *id})
	if err != nil {
		return err
	}
	if full {
		return printJSON(f.payload)
	}
	var resp struct {
		Game struct {
			Name string `json:"name"`
		} `json:"game"`
		Artifacts []Artifact `json:"artifacts"`
	}
	if err := json.Unmarshal(f.payload, &resp); err != nil {
		return err
	}
	fmt.Printf("%s — %d artefacto(s)\n", resp.Game.Name, len(resp.Artifacts))
	for _, a := range resp.Artifacts {
		slot := a.Slot
		if slot == "" {
			slot = "-"
		}
		fmt.Printf("  %-5s slot %-4s %10s  %s  %s%s\n", a.Kind, slot, humanBytes(a.Size),
			time.Unix(a.Mtime, 0).Format("2006-01-02 15:04"), a.File, thumbMark(a.Thumb))
	}
	return nil
}

func cmdPull(args []string) error {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	id := fs.String("id", "", "id del juego")
	kind := fs.String("kind", "state", "state|sram|rtc")
	slot := fs.String("slot", "auto", "auto|0..9")
	out := fs.String("out", "", "archivo destino")
	fs.Parse(args)
	if *id == "" {
		return errors.New("falta -id")
	}
	name := *out
	if name == "" {
		switch *kind {
		case "sram":
			name = *id + "-sram.srm"
		case "rtc":
			name = *id + "-rtc.rtc"
		default:
			name = fmt.Sprintf("%s-slot%s.state", *id, *slot)
		}
	}
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()

	c, err := dial(*addr)
	if err != nil {
		f.Close()
		return err
	}
	defer c.Close()

	meta, err := c.getFile(opGet, map[string]any{"id": *id, "kind": *kind, "slot": *slot}, f)
	if err != nil {
		os.Remove(name)
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if mt, ok := meta["mtime"].(float64); ok && mt > 0 {
		t := time.Unix(int64(mt), 0)
		_ = os.Chtimes(name, t, t)
		fmt.Printf("guardado %s (%v bytes) etag=%v mtime=%s\n", name, meta["size"], meta["etag"], t.Format("2006-01-02 15:04"))
		return nil
	}
	fmt.Printf("guardado %s (%v bytes) etag=%v\n", name, meta["size"], meta["etag"])
	return nil
}

func cmdPush(args []string) error {
	fs := flag.NewFlagSet("push", flag.ExitOnError)
	id := fs.String("id", "", "id del juego")
	kind := fs.String("kind", "state", "state|sram|rtc")
	slot := fs.String("slot", "auto", "auto|0..9")
	in := fs.String("in", "", "archivo a subir")
	force := fs.Bool("force", false, "ignorar precondition")
	ifMatch := fs.String("ifmatch", "", "ETag esperado del destino")
	fs.Parse(args)
	if *id == "" || *in == "" {
		return errors.New("faltan -id y -in")
	}

	st, err := os.Stat(*in)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)

	c, err := dial(*addr)
	if err != nil {
		return err
	}
	defer c.Close()
	out, err := c.putFile(map[string]any{
		"id": *id, "kind": *kind, "slot": *slot,
		"size": st.Size(), "sha256": hex.EncodeToString(sum[:]),
		"mtime": st.ModTime().Unix(), "force": *force, "ifMatch": *ifMatch,
	}, data)
	if err != nil {
		return err
	}
	fmt.Printf("subido %s (%s) sha256=%s\n", *in, humanBytes(st.Size()), hex.EncodeToString(sum[:])[:16])
	return printJSON(mustJSON(out))
}

func cmdMedia(args []string) error {
	fs := flag.NewFlagSet("media", flag.ExitOnError)
	id := fs.String("id", "", "id del juego")
	typ := fs.String("type", "boxart", "boxart|thumb|marquee|video")
	out := fs.String("out", "", "archivo destino")
	fs.Parse(args)
	if *id == "" {
		return errors.New("falta -id")
	}
	c, err := dial(*addr)
	if err != nil {
		return err
	}
	defer c.Close()

	var buf bytes.Buffer
	meta, err := c.getFile(opMedia, map[string]any{"id": *id, "type": *typ}, &buf)
	if err != nil {
		return err
	}
	name := *out
	if name == "" {
		name = fmt.Sprintf("%s-%s", *id, *typ)
		if ct, _ := meta["contentType"].(string); ct == "image/png" {
			name += ".png"
		} else if ct == "image/jpeg" {
			name += ".jpg"
		} else if ct == "image/webp" {
			name += ".webp"
		}
	}
	if err := os.WriteFile(name, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Printf("guardado %s (%s)\n", name, humanBytes(int64(buf.Len())))
	return nil
}

func cmdRefresh(args []string) error {
	fs := flag.NewFlagSet("refresh", flag.ExitOnError)
	wait := fs.Bool("wait", false, "esperar al escaneo")
	fs.Parse(args)
	c, err := dial(*addr)
	if err != nil {
		return err
	}
	defer c.Close()
	f, err := c.call(opRefresh, map[string]any{"wait": *wait})
	if err != nil {
		return err
	}
	return printJSON(f.payload)
}

func cmdAudit(args []string) error {
	fs := flag.NewFlagSet("audit", flag.ExitOnError)
	dry := fs.Bool("dry", false, "solo informar, sin modificar nada")
	fs.Parse(args)
	c, err := dial(*addr)
	if err != nil {
		return err
	}
	defer c.Close()
	f, err := c.call(opAudit, map[string]any{"dry": *dry})
	if err != nil {
		return err
	}
	return printJSON(f.payload)
}

func cmdLaunch(args []string, op byte) error {
	fs := flag.NewFlagSet("launch", flag.ExitOnError)
	id := fs.String("id", "", "id del juego")
	fs.Parse(args)
	c, err := dial(*addr)
	if err != nil {
		return err
	}
	defer c.Close()
	f, err := c.call(op, map[string]any{"id": *id})
	if err != nil {
		return err
	}
	return printJSON(f.payload)
}

func cmdDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	id := fs.String("id", "", "id del juego")
	fs.Parse(args)
	c, err := dial(*addr)
	if err != nil {
		return err
	}
	defer c.Close()
	f, err := c.call(opDelete, map[string]any{"id": *id})
	if err != nil {
		return err
	}
	return printJSON(f.payload)
}

func cmdMeta(args []string) error {
	fs := flag.NewFlagSet("meta", flag.ExitOnError)
	id := fs.String("id", "", "id del juego")
	desc := fs.String("desc", "", "descripción")
	dev := fs.String("developer", "", "desarrollador")
	img := fs.String("image", "", "imagen JPEG/PNG")
	fs.Parse(args)
	meta := map[string]any{"desc": *desc, "developer": *dev}
	if *img != "" {
		b, err := os.ReadFile(*img)
		if err != nil {
			return err
		}
		meta["image"] = base64.StdEncoding.EncodeToString(b)
	}
	c, err := dial(*addr)
	if err != nil {
		return err
	}
	defer c.Close()
	f, err := c.call(opSetMeta, map[string]any{"id": *id, "meta": meta})
	if err != nil {
		return err
	}
	return printJSON(f.payload)
}

func cmdScrape(args []string) error {
	fs := flag.NewFlagSet("scrape", flag.ExitOnError)
	id := fs.String("id", "", "id del juego")
	lang := fs.String("lang", "es", "idioma de la descripción")
	fs.Parse(args)
	c, err := dial(*addr)
	if err != nil {
		return err
	}
	defer c.Close()
	f, err := c.call(opScrape, map[string]any{"id": *id, "lang": *lang, "ss": map[string]any{}})
	if err != nil {
		return err
	}
	return printJSON(f.payload)
}

func cmdStatus() error {
	c, err := dial(*addr)
	if err != nil {
		return err
	}
	defer c.Close()
	f, err := c.call(opStatus, nil)
	if err != nil {
		return err
	}
	return printControlState(f.payload)
}

func cmdControl(args []string) error {
	fs := flag.NewFlagSet("control", flag.ExitOnError)
	volume := fs.Int("volume", -1, "volumen 0..100")
	brightness := fs.Int("brightness", -1, "brillo 1..100")
	mute := fs.Bool("mute", false, "silenciar")
	unmute := fs.Bool("unmute", false, "quitar silencio")
	fs.Parse(args)

	if *mute && *unmute {
		return errors.New("elige -mute o -unmute, no ambos")
	}
	if *volume < 0 && *brightness < 0 && !*mute && !*unmute {
		return cmdStatus()
	}

	c, err := dial(*addr)
	if err != nil {
		return err
	}
	defer c.Close()

	do := func(action string, value int, m *bool) error {
		req := map[string]any{"action": action, "value": value}
		if m != nil {
			req["mute"] = *m
		}
		f, err := c.call(opControl, req)
		if err != nil {
			return err
		}
		return printControlState(f.payload)
	}
	if *volume >= 0 {
		if err := do("volume", *volume, nil); err != nil {
			return err
		}
	}
	if *brightness >= 0 {
		if err := do("brightness", *brightness, nil); err != nil {
			return err
		}
	}
	if *mute || *unmute {
		m := *mute
		if err := do("mute", 0, &m); err != nil {
			return err
		}
	}
	return nil
}

func printControlState(body []byte) error {
	var st struct {
		Volume     *int   `json:"volume"`
		Muted      *bool  `json:"muted"`
		Brightness *int   `json:"brightness"`
		Battery    *int   `json:"battery"`
		Charging   *bool  `json:"charging"`
		Audio      string `json:"audio"`
		Backlight  string `json:"backlightDriver"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal(body, &st); err != nil {
		return err
	}
	if st.Volume != nil {
		muteTxt := ""
		if st.Muted != nil && *st.Muted {
			muteTxt = " · silenciado"
		}
		fmt.Printf("%-10s %3d%%  (%s%s)\n", "volumen", *st.Volume, st.Audio, muteTxt)
	}
	if st.Brightness != nil {
		fmt.Printf("%-10s %3d%%  (%s)\n", "brillo", *st.Brightness, st.Backlight)
	}
	if st.Battery != nil {
		chg := ""
		if st.Charging != nil && *st.Charging {
			chg = " · cargando"
		}
		fmt.Printf("%-10s %3d%%%s\n", "batería", *st.Battery, chg)
	}
	if st.Error != "" {
		fmt.Printf("%-10s %s\n", "aviso", st.Error)
	}
	return nil
}

// ------------------------------------------------------------- discovery ---
func cmdDiscover(args []string) error {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	group := fs.String("group", "239.255.42.99", "grupo multicast")
	port := fs.Int("port", 47820, "puerto multicast")
	wait := fs.Duration("timeout", 5*time.Second, "tiempo de escucha")
	ifaceName := fs.String("iface", "", "interfaz de red (nombre o IP)")
	fs.Parse(args)

	ip := net.ParseIP(*group).To4()
	if ip == nil {
		return errors.New("grupo inválido")
	}
	ifi := pickIface(*ifaceName)
	conn, err := net.ListenMulticastUDP("udp4", ifi, &net.UDPAddr{IP: ip, Port: *port})
	if err != nil {
		return fmt.Errorf("no se pudo escuchar multicast: %w", err)
	}
	defer conn.Close()
	_ = conn.SetReadBuffer(64 << 10)

	dst := &net.UDPAddr{IP: ip, Port: *port}
	var laddr *net.UDPAddr
	if ifi != nil {
		if v4 := firstIPv4(ifi); v4 != nil {
			laddr = &net.UDPAddr{IP: v4}
		}
	}
	if send, err := net.ListenUDP("udp4", laddr); err == nil {
		defer send.Close()
		probe := map[string]any{"magic": "MULTIRK1", "type": "probe", "id": "multirkctl", "name": "multirkctl", "ts": time.Now().Unix()}
		b, _ := json.Marshal(probe)
		_, _ = send.WriteToUDP(b, dst)
	}

	ifName := "auto"
	if ifi != nil {
		ifName = ifi.Name
	}
	fmt.Printf("escuchando %s:%d durante %s… (iface %s)\n", ip, *port, *wait, ifName)
	deadline := time.Now().Add(*wait)
	seen := map[string]bool{}
	buf := make([]byte, 4096)
	for {
		_ = conn.SetReadDeadline(deadline)
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		var p struct {
			Magic     string `json:"magic"`
			Type      string `json:"type"`
			ID        string `json:"id"`
			Name      string `json:"name"`
			Version   string `json:"version"`
			Protocol  string `json:"protocol"`
			Port      int    `json:"port"`
			Systems   int    `json:"systems"`
			Games     int    `json:"games"`
			WithState int    `json:"withState"`
		}
		if json.Unmarshal(buf[:n], &p) != nil || p.Magic != "MULTIRK1" {
			continue
		}
		if p.Type != "announce" || seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		fmt.Printf("  %-22s %s:%d  v%s  %s  %d sistemas · %d juegos · %d con estado\n",
			p.Name, src.IP, p.Port, p.Version, strings.ToUpper(p.Protocol), p.Systems, p.Games, p.WithState)
	}
	if len(seen) == 0 {
		fmt.Println("  (ninguna consola respondió)")
	}
	return nil
}

// --------------------------------------------------------------- install ---

func cmdInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	host := fs.String("host", "", "IP o host de la consola")
	user := fs.String("user", "ark", "usuario SSH")
	sshPort := fs.Int("port", 22, "puerto SSH")
	key := fs.String("key", "", "clave privada SSH")
	password := fs.String("password", "", "contraseña SSH (ArkOS por defecto: ark)")
	name := fs.String("name", "", "nombre visible (por defecto el host)")
	roms := fs.String("roms", "/roms", "raíz de roms en la consola")
	dataDir := fs.String("data", "", "carpeta de datos (por defecto ~/.config/multirk)")
	binaryPath := fs.String("binary", "", "binario linux a subir (autodetecta arquitectura)")
	service := fs.String("service", "auto", "auto|systemd|none")
	start := fs.Bool("start", true, "arrancar al terminar")
	httpDebug := fs.Bool("http", false, "dejar HTTP de depuración en :47821")
	fs.Parse(args)

	if *host == "" {
		return errors.New("falta -host (ej. 192.168.1.42)")
	}
	if strings.Contains(*host, "@") {
		parts := strings.SplitN(*host, "@", 2)
		*user = parts[0]
		*host = parts[1]
	}
	if *name == "" {
		*name = strings.Split(*host, ".")[0]
	}
	r := remote{host: *host, user: *user, port: *sshPort, key: *key, password: *password}
	if r.password != "" {
		r.sshpass = findSshpass()
		if r.sshpass == "" {
			return errors.New("no encuentro sshpass; instálalo o usa -key con una clave SSH")
		}
	}

	if _, err := exec.LookPath("ssh"); err != nil {
		return errors.New("no encuentro el cliente ssh en el PATH")
	}
	if _, err := exec.LookPath("scp"); err != nil {
		return errors.New("no encuentro scp en el PATH")
	}

	// Fail fast with a clear message.
	if _, err := r.run("true"); err != nil {
		return fmt.Errorf("no se pudo entrar por SSH a %s@%s.\n"+
			"Usa -password ark (ArkOS) o configura una clave: ssh-copy-id %s@%s",
			r.user, r.host, r.user, r.host)
	}

	archOut, err := r.run("uname -m")
	if err != nil {
		return err
	}
	arch := strings.TrimSpace(archOut)
	bin, err := ensureBinary(arch, *binaryPath)
	if err != nil {
		return err
	}
	fmt.Printf("consola %s (%s) · binario %s\n", *host, arch, filepath.Base(bin))

	homeOut, err := r.run("echo $HOME")
	if err != nil {
		return err
	}
	home := strings.TrimSpace(homeOut)
	if home == "" || !strings.HasPrefix(home, "/") {
		return fmt.Errorf("respuesta rara al leer $HOME: %q", home)
	}
	if *dataDir == "" {
		*dataDir = home + "/.config/multirk"
	}

	if _, err := r.run("mkdir -p " + shq(home+"/multirk") + " " + shq(*dataDir)); err != nil {
		return err
	}
	if err := r.upload(bin, home+"/multirk/multirkd.new"); err != nil {
		return err
	}
	if _, err := r.run("chmod +x " + shq(home+"/multirk/multirkd.new") + " && mv -f " + shq(home+"/multirk/multirkd.new") + " " + shq(home+"/multirk/multirkd")); err != nil {
		return err
	}

	cfg := map[string]any{
		"name":            *name,
		"romsRoot":        *roms,
		"dataDir":         *dataDir,
		"protocol":        "tcp",
		"listen":          ":47820",
		"auth":            true,
		"httpListen":      "",
		"scanIntervalSec": 1800,
		"stateRefreshSec": 60,
		"multicast": map[string]any{
			"group": "239.255.42.99", "port": 47820, "intervalSec": 10,
		},
	}
	if *httpDebug {
		cfg["httpListen"] = ":47821"
	}
	cfgBytes, _ := json.MarshalIndent(cfg, "", "  ")
	tmpCfg, err := os.CreateTemp("", "multirk-config-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmpCfg.Name())
	if _, err := tmpCfg.Write(append(cfgBytes, '\n')); err != nil {
		return err
	}
	tmpCfg.Close()
	if err := r.upload(tmpCfg.Name(), *dataDir+"/config.json"); err != nil {
		return err
	}

	svcMode := *service
	if svcMode == "auto" {
		if out, err := r.run("command -v systemctl >/dev/null 2>&1 && echo yes || echo no"); err == nil && strings.TrimSpace(out) == "yes" {
			if out, err := r.run("sudo -n true >/dev/null 2>&1 && echo yes || echo no"); err == nil && strings.TrimSpace(out) == "yes" {
				svcMode = "systemd"
			} else {
				svcMode = "none"
				fmt.Println("aviso: systemd existe pero sudo pide contraseña; instalo el arranque manual")
			}
		} else {
			svcMode = "none"
		}
	}

	startCmd := fmt.Sprintf("%s/multirkd -config %s/config.json", home+"/multirk", *dataDir)
	if svcMode == "systemd" {
		if err := installSystemd(r, home, *dataDir, *user, *roms); err != nil {
			fmt.Printf("aviso: systemd falló (%v); usando arranque manual\n", err)
			svcMode = "none"
		}
	}
	if svcMode != "systemd" {
		script := fmt.Sprintf("#!/bin/sh\n# MULTIRK agent\npkill -f multirkd 2>/dev/null || true\nnohup %s >/dev/shm/multirkd.log 2>&1 &\n", startCmd)
		if _, err := r.run("cat > " + shq(home+"/multirk/start.sh") + " <<'EOF'\n" + script + "EOF\nchmod +x " + shq(home+"/multirk/start.sh")); err != nil {
			return err
		}
		if *start {
			if _, err := r.run(shq(home+"/multirk/start.sh")); err != nil {
				return err
			}
		}
		fmt.Println("servicio: arranque manual (start.sh). En ArkOS añádelo a tu autostart.")
	} else {
		fmt.Println("servicio: systemd multirkd activo y habilitado")
	}

	time.Sleep(700 * time.Millisecond)
	fmt.Printf("\nlisto. Prueba:\n  multirkctl -addr %s:47820 info\n", *host)
	return nil
}

type remote struct {
	host, user string
	port       int
	key        string
	password   string
	sshpass    string
}

func (r remote) target() string { return r.user + "@" + r.host }

func findSshpass() string {
	if p, err := exec.LookPath("sshpass"); err == nil {
		return p
	}
	for _, c := range []string{
		`C:\Program Files\win-sshpass\sshpass.exe`,
		`C:\Program Files\sshpass\sshpass.exe`,
		`/usr/bin/sshpass`,
		`/usr/local/bin/sshpass`,
	} {
		if isFile(c) {
			return c
		}
	}
	return ""
}

func (r remote) sshOptions() []string {
	args := []string{
		"-o", "ConnectTimeout=8",
		"-o", "StrictHostKeyChecking=accept-new",
	}
	if r.password != "" {
		args = append(args,
			"-o", "PreferredAuthentications=password",
			"-o", "PubkeyAuthentication=no",
			"-o", "NumberOfPasswordPrompts=1",
		)
	} else {
		args = append(args, "-o", "BatchMode=yes")
	}
	if r.key != "" {
		args = append(args, "-i", r.key)
	}
	return args
}

func (r remote) sshArgs(command string) []string {
	args := append(r.sshOptions(), "-p", strconv.Itoa(r.port))
	return append(args, r.target(), command)
}

func (r remote) run(command string) (string, error) {
	var cmd *exec.Cmd
	if r.password != "" {
		args := append([]string{"-p", r.password, "ssh"}, r.sshArgs(command)...)
		cmd = exec.Command(r.sshpass, args...)
	} else {
		cmd = exec.Command("ssh", r.sshArgs(command)...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("ssh: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (r remote) upload(local, remotePath string) error {
	args := append(r.sshOptions(), "-P", strconv.Itoa(r.port))
	args = append(args, local, r.target()+":"+remotePath)
	var cmd *exec.Cmd
	if r.password != "" {
		cmd = exec.Command(r.sshpass, append([]string{"-p", r.password, "scp"}, args...)...)
	} else {
		cmd = exec.Command("scp", args...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scp: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func installSystemd(r remote, home, dataDir, user, roms string) error {
	unit := fmt.Sprintf(`[Unit]
Description=MULTIRK sync agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=%s
ExecStart=%s/multirk/multirkd -config %s/config.json
Restart=on-failure
RestartSec=5
Nice=10
IOSchedulingClass=idle
MemoryMax=64M
CPUQuota=25%%
ProtectSystem=full
PrivateTmp=true
Environment=GOMEMLIMIT=48MiB
Environment=GOGC=50
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`, user, home, dataDir)

	tmp, err := os.CreateTemp("", "multirkd-*.service")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(unit); err != nil {
		return err
	}
	tmp.Close()
	if err := r.upload(tmp.Name(), "/tmp/multirkd.service"); err != nil {
		return err
	}
	if _, err := r.run("sudo -n mv /tmp/multirkd.service /etc/systemd/system/multirkd.service && " +
		"sudo -n systemctl daemon-reload && " +
		"sudo -n systemctl enable multirkd >/dev/null 2>&1 && " +
		"sudo -n systemctl restart multirkd"); err != nil {
		return err
	}
	_ = roms
	return nil
}

func ensureBinary(arch, explicit string) (string, error) {
	if explicit != "" {
		if !isFile(explicit) {
			return "", fmt.Errorf("no existe el binario %s", explicit)
		}
		return explicit, nil
	}
	goarch := "arm64"
	if strings.HasPrefix(arch, "armv7") || strings.HasPrefix(arch, "armv8l") || arch == "arm" {
		goarch = "arm"
	}
	name := fmt.Sprintf("multirkd-linux-%s", goarch)
	candidates := binaryCandidates(name)
	for _, c := range candidates {
		if isFile(c) {
			return c, nil
		}
	}

	mod := findModuleDir()
	if mod == "" {
		return "", fmt.Errorf("no encuentro %s y no estoy dentro del módulo Go para compilarlo", name)
	}
	out := filepath.Join(mod, "dist", name)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	fmt.Printf("compilando %s para linux/%s…\n", name, goarch)
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", out, ".")
	cmd.Dir = mod
	env := append(os.Environ(), "GOOS=linux", "GOARCH="+goarch, "CGO_ENABLED=0")
	if goarch == "arm" {
		env = append(env, "GOARM=7")
	}
	cmd.Env = env
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("falló la compilación cruzada: %w", err)
	}
	return out, nil
}

func binaryCandidates(name string) []string {
	var out []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		out = append(out, filepath.Join(dir, name), filepath.Join(dir, "..", "dist", name))
	}
	if wd, err := os.Getwd(); err == nil {
		out = append(out, filepath.Join(wd, "dist", name), filepath.Join(wd, "..", "dist", name))
	}
	if mod := findModuleDir(); mod != "" {
		out = append(out, filepath.Join(mod, "dist", name))
	}
	return out
}

func findModuleDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for i := 0; i < 5; i++ {
		if isFile(filepath.Join(dir, "go.mod")) {
			return dir
		}
		if isFile(filepath.Join(dir, "daemon", "go.mod")) {
			return filepath.Join(dir, "daemon")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// ------------------------------------------------------------------ utils ---

func printJSON(body []byte) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, body, "", "  "); err != nil {
		fmt.Println(string(body))
		return nil
	}
	fmt.Println(buf.String())
	return nil
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func shq(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func thumbMark(has bool) string {
	if has {
		return "  (captura)"
	}
	return ""
}

func hostname() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	if runtime.GOOS == "windows" {
		return "Windows"
	}
	return "multirkctl"
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func pickIface(want string) *net.Interface {
	want = strings.ToLower(strings.TrimSpace(want))
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	if want != "" {
		for i := range ifaces {
			ifi := &ifaces[i]
			if strings.ToLower(ifi.Name) == want {
				return ifi
			}
			if ip := net.ParseIP(want); ip != nil {
				for _, a := range addrsOf(ifi) {
					var have net.IP
					switch v := a.(type) {
					case *net.IPNet:
						have = v.IP
					case *net.IPAddr:
						have = v.IP
					}
					if have != nil && have.Equal(ip) {
						return ifi
					}
				}
			}
		}
		return nil
	}
	if want == "" {
		if ifi := bestLANInterface(); ifi != nil {
			return ifi
		}
	}
	if conn, err := net.Dial("udp4", "1.1.1.1:53"); err == nil {
		la, _ := conn.LocalAddr().(*net.UDPAddr)
		conn.Close()
		if la != nil {
			for i := range ifaces {
				for _, a := range addrsOf(&ifaces[i]) {
					if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.Equal(la.IP) {
						return &ifaces[i]
					}
				}
			}
		}
	}
	return nil
}

func bestLANInterface() *net.Interface {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	bestScore := -1
	var best *net.Interface
	for i := range ifaces {
		ifi := &ifaces[i]
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagLoopback != 0 || ifi.Flags&net.FlagMulticast == 0 {
			continue
		}
		if isVirtualIface(ifi.Name) {
			continue
		}
		ip, prefix := firstIPv4Prefix(ifi)
		if ip == nil || prefix >= 32 {
			continue
		}
		score := 10
		name := strings.ToLower(ifi.Name)
		switch {
		case strings.HasPrefix(name, "wl"):
			score += 100
		case strings.HasPrefix(name, "eth"), strings.HasPrefix(name, "en"):
			score += 90
		case strings.HasPrefix(name, "usb"):
			score += 50
		}
		if ip.IsPrivate() {
			score += 5
		}
		if score > bestScore {
			bestScore = score
			best = ifi
		}
	}
	if best != nil {
		return best
	}
	return nil
}

func isVirtualIface(name string) bool {
	n := strings.ToLower(name)
	for _, bad := range []string{"vethernet", "vmware", "virtualbox", "docker", "veth", "br-", "tun", "tap", "tailscale", "zerotier", "warp", "bluetooth", "npcap", "loopback"} {
		if strings.Contains(n, bad) {
			return true
		}
	}
	return false
}

func firstIPv4Prefix(ifi *net.Interface) (net.IP, int) {
	for _, a := range addrsOf(ifi) {
		if ipnet, ok := a.(*net.IPNet); ok {
			if v4 := ipnet.IP.To4(); v4 != nil && !v4.IsLoopback() {
				ones, _ := ipnet.Mask.Size()
				return v4, ones
			}
		}
	}
	return nil, 0
}

func addrsOf(ifi *net.Interface) []net.Addr {
	addrs, err := ifi.Addrs()
	if err != nil {
		return nil
	}
	return addrs
}

func firstIPv4(ifi *net.Interface) net.IP {
	for _, a := range addrsOf(ifi) {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil {
			continue
		}
		if v4 := ip.To4(); v4 != nil && !v4.IsLoopback() {
			return v4
		}
	}
	return nil
}

type Artifact struct {
	Kind  string `json:"kind"`
	Slot  string `json:"slot"`
	File  string `json:"file"`
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"`
	Thumb bool   `json:"thumb"`
}
