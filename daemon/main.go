package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var version = "0.1.0"

func main() {
	var (
		configPath = flag.String("config", "", "ruta a config.json")
		roms       = flag.String("roms", "", "raíz de roms (normalmente /roms)")
		dataDir    = flag.String("data", "", "carpeta de datos (device id, token)")
		listen     = flag.String("listen", "", "dirección, ej. :47820")
		proto      = flag.String("proto", "", "tcp (rápido) o http (depuración)")
		httpListen = flag.String("http-listen", "", "dirección HTTP opcional, ej. :47821")
		name       = flag.String("name", "", "nombre visible de la consola")
		iface      = flag.String("iface", "", "interfaz de red para multicast (nombre o IP)")
		noAuth     = flag.Bool("no-auth", false, "desactiva el token (solo pruebas)")
		scanOnly   = flag.Bool("scan", false, "escanea y muestra un resumen, sin servir")
		showVer    = flag.Bool("version", false, "muestra la versión")
	)
	flag.Parse()

	if *showVer {
		fmt.Printf("multirkd %s\n", version)
		return
	}

	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = filepath.Join(defaultDataDir(), "config.json")
	}
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	cfg.Path = cfgPath
	if *roms != "" {
		cfg.RomsRoot = *roms
	}
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}
	if *listen != "" {
		cfg.Listen = *listen
	}
	if *proto != "" {
		cfg.Protocol = *proto
	}
	if *httpListen != "" {
		cfg.HTTPListen = *httpListen
	}
	if *name != "" {
		cfg.Name = *name
	}
	if *iface != "" {
		cfg.Multicast.Iface = *iface
	}
	if *noAuth {
		cfg.Auth = false
	}
	cfg.Ensure()

	log.SetFlags(log.Ltime)

	rt := LoadRuntime(cfg.DataDir)

	ix := NewIndex(cfg)
	start := time.Now()
	log.Printf("scan: leyendo %s …", cfg.RomsRoot)
	if err := ix.Scan(); err != nil {
		log.Printf("scan inicial: %v", err)
	}

	if *scanOnly {
		snap := ix.Snapshot()
		if snap == nil {
			fmt.Println("sin catálogo")
			os.Exit(2)
		}
		fmt.Printf("%s: %d sistemas, %d juegos, %d con estado (%s)\n",
			cfg.RomsRoot, len(snap.Systems), len(snap.Games), countWithState(snap.Games),
			time.Since(start).Round(time.Millisecond))
		for _, sys := range snap.Systems {
			fmt.Printf("  %-14s %-30s %5d juegos  %4d con estado\n", sys.Key, sys.Name, sys.Games, sys.WithState)
		}
		return
	}

	printBanner(cfg, rt, ix)

	srv := NewServer(cfg, ix, rt)
	var httpSrv *http.Server
	var tcpLn net.Listener

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.Protocol == "http" {
		httpSrv = &http.Server{
			Addr:              cfg.HTTPListen,
			Handler:           srv.Routes(),
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       90 * time.Second,
		}
		go func() {
			log.Printf("http: escuchando en %s (protocolo de depuración)", cfg.HTTPListen)
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("http: %v", err)
			}
		}()
	} else {
		ln, err := net.Listen("tcp", cfg.Listen)
		if err != nil {
			log.Fatalf("tcp: %v", err)
		}
		tcpLn = ln
		log.Printf("tcp: escuchando en %s (MULTIRK/2, frames binarios)", cfg.Listen)
		tcp := NewTCPServer(cfg, ix, rt)
		go func() {
			if err := tcp.Serve(ctx, ln); err != nil {
				log.Printf("tcp: %v", err)
			}
		}()
		if cfg.HTTPListen != "" {
			httpSrv = &http.Server{
				Addr:              cfg.HTTPListen,
				Handler:           srv.Routes(),
				ReadHeaderTimeout: 5 * time.Second,
			}
			go func() {
				log.Printf("http: escuchando en %s (solo depuración)", cfg.HTTPListen)
				if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Printf("http: %v", err)
				}
			}()
		}
	}
	go startDiscovery(ctx, cfg, ix, rt)

	go func() {
		t := time.NewTicker(time.Duration(cfg.StateRefreshSec) * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				ix.RefreshStates()
			}
		}
	}()

	go func() {
		t := time.NewTicker(time.Duration(cfg.ScanIntervalSec) * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := ix.Scan(); err != nil {
					log.Printf("scan periódico: %v", err)
				}
			}
		}
	}()

	<-ctx.Done()
	log.Printf("apagando…")
	if tcpLn != nil {
		_ = tcpLn.Close()
	}
	if httpSrv != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}
}

func printBanner(cfg *Config, rt *Runtime, ix *Index) {
	systems, games, withState := 0, 0, 0
	if snap := ix.Snapshot(); snap != nil {
		systems = len(snap.Systems)
		games = len(snap.Games)
		withState = countWithState(snap.Games)
	}
	ips := strings.Join(lanIPs(), ", ")
	if ips == "" {
		ips = "sin IP de LAN"
	}
	log.Printf("multirkd %s · %s · id %s", version, cfg.Name, rt.DeviceID)
	log.Printf("roms:    %s", cfg.RomsRoot)
	log.Printf("states:  %s", joinOr(cfg.StateDirs, "—"))
	log.Printf("saves:   %s", joinOr(cfg.SaveDirs, "—"))
	log.Printf("red:     %s · escucha %s  (auth=%v)", ips, cfg.Listen, cfg.Auth)
	log.Printf("proto:   %s", strings.ToUpper(cfg.Protocol))
	log.Printf("multi:   %s:%d", cfg.Multicast.Group, cfg.Multicast.Port)
	log.Printf("catálogo:%d sistemas · %d juegos · %d con estado", systems, games, withState)
	if len(cfg.StateDirs) == 0 {
		log.Printf("aviso: no se encontró la carpeta de savestates; define stateDirs en %s", cfg.Path)
	}
}

func joinOr(list []string, fallback string) string {
	if len(list) == 0 {
		return fallback
	}
	return strings.Join(list, ", ")
}
