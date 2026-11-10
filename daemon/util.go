package main

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func expandPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return filepath.Clean(p)
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func appendUnique(list []string, values ...string) []string {
	seen := map[string]bool{}
	for _, v := range list {
		seen[v] = true
	}
	for _, v := range values {
		v = filepath.Clean(v)
		if v == "" || v == "." || seen[v] {
			continue
		}
		if !isDir(v) {
			continue
		}
		seen[v] = true
		list = append(list, v)
	}
	return list
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		// crypto/rand failing is fatal in practice; fall back to a fixed-ish
		// value only so the daemon can still report a useful error.
		for i := range b {
			b[i] = byte(i * 7)
		}
	}
	return hex.EncodeToString(b)
}

func sha1Hex(parts ...string) string {
	h := sha1.New()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte{0})
		}
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func gameID(system, relPath string) string {
	return sha1Hex(strings.ToLower(system), strings.ToLower(relPath))[:16]
}

// prettyName turns "Super_Mario_World (USA)" into "Super Mario World (USA)".
func prettyName(base string) string {
	return strings.TrimSpace(strings.ReplaceAll(base, "_", " "))
}

func splitBaseExt(name string) (base, ext string) {
	ext = strings.ToLower(filepath.Ext(name))
	base = strings.TrimSuffix(name, filepath.Ext(name))
	return base, ext
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
