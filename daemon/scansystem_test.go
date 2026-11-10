package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanSystemOnlyTouchesThatSystem(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"snes/Uno.sfc", "snes/Dos.sfc", "gba/Tres.gba"} {
		p := filepath.Join(root, filepath.FromSlash(f))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("x"), 0o644)
	}
	cfg := &Config{RomsRoot: root}
	ix := NewIndex(cfg)
	if err := ix.Scan(); err != nil {
		t.Fatal(err)
	}
	before := ix.Snapshot()
	if len(before.Games) != 3 {
		t.Fatalf("juegos iniciales=%d", len(before.Games))
	}

	// cambios en snes (añadir y quitar) y en gba (añadir): solo snes se reescanea
	_ = os.WriteFile(filepath.Join(root, "snes", "Nuevo.sfc"), []byte("x"), 0o644)
	_ = os.Remove(filepath.Join(root, "snes", "Uno.sfc"))
	_ = os.WriteFile(filepath.Join(root, "gba", "Cuatro.gba"), []byte("x"), 0o644)

	if err := ix.ScanSystem("snes"); err != nil {
		t.Fatal(err)
	}
	after := ix.Snapshot()
	count := map[string]int{}
	names := map[string]bool{}
	for _, g := range after.Games {
		count[g.System]++
		names[g.File] = true
	}
	if count["snes"] != 2 || !names["Nuevo.sfc"] || names["Uno.sfc"] {
		t.Fatalf("snes mal reescaneado: %v %v", count, names)
	}
	if count["gba"] != 1 || names["Cuatro.gba"] {
		t.Fatalf("gba no debía reescanearse: %v %v", count, names)
	}
}
