package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"strings"
	"time"
)

const discoveryMagic = "MULTIRK1"

// DiscoveryPacket is intentionally tiny: the app only needs enough to show a
// card and know where to POST /v1/pair.
type DiscoveryPacket struct {
	Magic     string `json:"magic"`
	Type      string `json:"type"` // announce | probe | bye
	ID        string `json:"id"`
	Name      string `json:"name"`
	Model     string `json:"model"`
	Version   string `json:"version"`
	API       string `json:"api"`
	Protocol  string `json:"protocol"`
	Port      int    `json:"port"`
	PairOpen  bool   `json:"pairOpen"`
	Systems   int    `json:"systems"`
	Games     int    `json:"games"`
	WithState int    `json:"withState"`
	TS        int64  `json:"ts"`
}

func startDiscovery(ctx context.Context, cfg *Config, ix *Index, rt *Runtime) {
	group := net.ParseIP(cfg.Multicast.Group).To4()
	if group == nil {
		log.Printf("discovery: grupo multicast inválido %q", cfg.Multicast.Group)
		return
	}
	ifi := pickInterface(cfg.Multicast.Iface)

	conn, err := net.ListenMulticastUDP("udp4", ifi, &net.UDPAddr{IP: group, Port: cfg.Multicast.Port})
	if err != nil {
		log.Printf("discovery: no se pudo escuchar en %s:%d: %v", group, cfg.Multicast.Port, err)
		log.Printf("discovery: la app podrá conectarse igualmente por IP manual")
		return
	}
	defer conn.Close()
	_ = conn.SetReadBuffer(64 << 10)

	var laddr *net.UDPAddr
	if ifi != nil {
		if ip := firstIPv4(ifi); ip != nil {
			laddr = &net.UDPAddr{IP: ip}
		}
	}
	send, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		log.Printf("discovery: no se pudo abrir socket de envío: %v", err)
		return
	}
	defer send.Close()
	dst := &net.UDPAddr{IP: group, Port: cfg.Multicast.Port}

	sendPacket := func(typ string, to *net.UDPAddr) {
		snap := ix.Snapshot()
		p := DiscoveryPacket{
			Magic:    discoveryMagic,
			Type:     typ,
			ID:       rt.DeviceID,
			Name:     cfg.Name,
			Model:    hardwareModel(),
			Version:  version,
			API:      "/v1",
			Protocol: cfg.Protocol,
			Port:     portOf(cfg.Listen),
			PairOpen: true,
			TS:       time.Now().Unix(),
		}
		if snap != nil {
			p.Systems = len(snap.Systems)
			p.Games = len(snap.Games)
			p.WithState = countWithState(snap.Games)
		}
		b, err := json.Marshal(p)
		if err != nil {
			return
		}
		target := dst
		if to != nil {
			target = to
		}
		if _, err := send.WriteToUDP(b, target); err != nil {
			log.Printf("discovery: envío falló: %v", err)
		}
	}

	interval := time.Duration(cfg.Multicast.IntervalSec) * time.Second
	log.Printf("discovery: anunciando en %s:%d cada %s", group, cfg.Multicast.Port, interval)

	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		sendPacket("probe", nil)
		sendPacket("announce", nil)
		for {
			select {
			case <-ctx.Done():
				sendPacket("bye", nil)
				return
			case <-t.C:
				sendPacket("announce", nil)
			}
		}
	}()

	buf := make([]byte, 4096)
	for {
		if ctx.Err() != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		var p DiscoveryPacket
		if err := json.Unmarshal(buf[:n], &p); err != nil || p.Magic != discoveryMagic {
			continue
		}
		if p.ID == rt.DeviceID {
			continue
		}
		if p.Type == "probe" {
			sendPacket("announce", src)
		}
	}
}

func pickInterface(want string) *net.Interface {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	want = strings.ToLower(strings.TrimSpace(want))
	if want == "" {
		return bestLANInterface()
	}
	for i := range ifaces {
		ifi := ifaces[i]
		if strings.EqualFold(ifi.Name, want) {
			return &ifi
		}
		if ip := net.ParseIP(want); ip != nil {
			for _, a := range addrsOf(&ifi) {
				var have net.IP
				switch v := a.(type) {
				case *net.IPNet:
					have = v.IP
				case *net.IPAddr:
					have = v.IP
				}
				if have != nil && have.Equal(ip) {
					return &ifi
				}
			}
		}
	}
	return nil
}

// bestLANInterface avoids VPN/WSL/virtual adapters that either cannot carry
// multicast or would send it to the wrong network. On a handheld it picks
// wlan0; on a dev box it picks the real Ethernet/Wi-Fi NIC.
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
	return defaultRouteInterface()
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

// defaultRouteInterface picks the NIC the OS would use for internet traffic.
// On a handheld it is wlan0; on a dev machine it avoids VPN/WSL adapters that
// cannot carry multicast.
func defaultRouteInterface() *net.Interface {
	conn, err := net.Dial("udp4", "1.1.1.1:53")
	if err != nil {
		return nil
	}
	defer conn.Close()
	la, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for i := range ifaces {
		for _, a := range addrsOf(&ifaces[i]) {
			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.Equal(la.IP) {
				return &ifaces[i]
			}
		}
	}
	return nil
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

func lanIPs() []string {
	var out []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for i := range ifaces {
		ifi := &ifaces[i]
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		if ip := firstIPv4(ifi); ip != nil {
			out = append(out, ip.String())
		}
	}
	return out
}

