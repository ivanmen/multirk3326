package main

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Client struct {
	Name    string `json:"name"`
	Token   string `json:"token"`
	Created int64  `json:"created"`
}

type pairAttempt struct {
	count int
	reset time.Time
}

type Runtime struct {
	DeviceID string   `json:"deviceId"`
	Clients  []Client `json:"clients"`

	mu       sync.Mutex
	path     string
	attempts map[string]*pairAttempt
}

func LoadRuntime(dataDir string) *Runtime {
	rt := &Runtime{
		path:     filepath.Join(dataDir, "runtime.json"),
		attempts: map[string]*pairAttempt{},
	}
	if b, err := os.ReadFile(rt.path); err == nil {
		_ = json.Unmarshal(b, rt)
	}
	changed := false
	if rt.DeviceID == "" {
		rt.DeviceID = randHex(8)
		changed = true
	}
	if changed {
		rt.save()
	}
	return rt
}

func (rt *Runtime) save() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	b, err := json.MarshalIndent(rt, "", "  ")
	if err != nil {
		return
	}
	tmp := rt.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		log.Printf("runtime: %v", err)
		return
	}
	if err := os.Rename(tmp, rt.path); err != nil {
		log.Printf("runtime: %v", err)
	}
}

func (rt *Runtime) Save() { rt.save() }

func (rt *Runtime) DeviceName() string {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.DeviceID
}

func (rt *Runtime) allowAttempt(ip string) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	now := time.Now()
	a := rt.attempts[ip]
	if a == nil || now.After(a.reset) {
		rt.attempts[ip] = &pairAttempt{count: 1, reset: now.Add(time.Minute)}
		return true
	}
	if a.count >= 5 {
		return false
	}
	a.count++
	return true
}

func (rt *Runtime) AddClient(name string) Client {
	if strings.TrimSpace(name) == "" {
		name = "Android"
	}
	c := Client{Name: name, Token: randHex(32), Created: time.Now().Unix()}
	rt.mu.Lock()
	// Replace a client with the same name: reinstalling the app should not
	// accumulate tokens forever.
	filtered := rt.Clients[:0]
	for _, old := range rt.Clients {
		if old.Name != name {
			filtered = append(filtered, old)
		}
	}
	rt.Clients = append(filtered, c)
	rt.mu.Unlock()
	rt.save()
	return c
}

func (rt *Runtime) Authorized(header string) bool {
	token := ""
	if strings.HasPrefix(header, "Bearer ") {
		token = strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	} else if header != "" {
		token = strings.TrimSpace(header)
	}
	if token == "" {
		return false
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	for _, c := range rt.Clients {
		if subtle.ConstantTimeCompare([]byte(token), []byte(c.Token)) == 1 {
			return true
		}
	}
	return false
}

func (rt *Runtime) ClientCount() int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return len(rt.Clients)
}

