package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveRomAndMedia(t *testing.T) {
	root := t.TempDir()
	sys := filepath.Join(root, "ports")
	_ = os.MkdirAll(filepath.Join(sys, "images"), 0o755)
	rom := filepath.Join(sys, "Juego.sh")
	img := filepath.Join(sys, "images", "Juego.png")
	vid := filepath.Join(sys, "images", "Juego.mp4")
	_ = os.WriteFile(rom, []byte("rom"), 0o644)
	_ = os.WriteFile(img, []byte("img"), 0o644)
	_ = os.WriteFile(vid, []byte("vid"), 0o644)
	other := filepath.Join(sys, "Otro.sh")
	_ = os.WriteFile(other, []byte("otro"), 0o644)

	n, err := removeRom(root, "ports", "Juego.sh", []string{img, vid})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("media borrada=%d, esperaba 2", n)
	}
	for _, p := range []string{rom, img, vid} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("sigue existiendo %s", p)
		}
	}
	if b, _ := os.ReadFile(other); string(b) != "otro" {
		t.Fatal("se tocó otro archivo")
	}
	if _, err := os.Stat(filepath.Join(root, ".multirk-trash")); err == nil {
		t.Fatal("no debe existir ninguna papelera")
	}
}

func TestRemoveRomRefuses(t *testing.T) {
	root := t.TempDir()
	sys := filepath.Join(root, "ports")
	_ = os.MkdirAll(sys, 0o755)
	_ = os.WriteFile(filepath.Join(root, "fuera.txt"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(sys, "gamelist.xml"), []byte("<gameList/>"), 0o644)
	_ = os.MkdirAll(filepath.Join(sys, "carpeta"), 0o755)

	for _, rel := range []string{"../fuera.txt", "gamelist.xml", "carpeta", "noexiste.sh"} {
		if _, err := removeRom(root, "ports", rel, nil); err == nil {
			t.Fatalf("debía rechazar %q", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "fuera.txt")); err != nil {
		t.Fatal("se borró un archivo fuera del sistema")
	}
	if _, err := os.Stat(filepath.Join(sys, "gamelist.xml")); err != nil {
		t.Fatal("se borró el gamelist")
	}
}

func TestRemoveRomKeepsMediaOutsideSystem(t *testing.T) {
	root := t.TempDir()
	sys := filepath.Join(root, "ports")
	_ = os.MkdirAll(sys, 0o755)
	_ = os.WriteFile(filepath.Join(sys, "A.sh"), []byte("a"), 0o644)
	ext := filepath.Join(root, "compartida.png")
	_ = os.WriteFile(ext, []byte("img"), 0o644)
	n, err := removeRom(root, "ports", "A.sh", []string{ext})
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if _, err := os.Stat(ext); err != nil {
		t.Fatal("se borró media fuera del sistema")
	}
}

func TestRemoveGameBlockRealGamelists(t *testing.T) {
	files, _ := filepath.Glob("../testdata/gamelists/*.xml")
	for _, f := range files {
		b, _ := os.ReadFile(f)
		src := string(b)
		blocks := reGameBlock.FindAllString(src, -1)
		if len(blocks) < 2 {
			continue
		}
		rel := strings.TrimPrefix(xmlUnesc(rePathTag.FindStringSubmatch(blocks[1])[1]), "./")
		out, removed := removeGameBlock(src, rel)
		if !removed {
			t.Fatalf("%s: no encontró la entrada", filepath.Base(f))
		}
		n, err := countGames(out)
		if err != nil || n != len(blocks)-1 {
			t.Fatalf("%s: conteo %d (esperaba %d) err=%v", filepath.Base(f), n, len(blocks)-1, err)
		}
		rest := reGameBlock.FindAllString(out, -1)
		for i, j := 0, 0; i < len(blocks); i++ {
			if i == 1 {
				continue
			}
			if rest[j] != blocks[i] {
				t.Fatalf("%s: cambió otra entrada", filepath.Base(f))
			}
			j++
		}
		if strings.Contains(out, blocks[1]) {
			t.Fatalf("%s: la entrada sigue", filepath.Base(f))
		}
	}
}

func TestApplyGamelistRemoveBackupRestore(t *testing.T) {
	b, err := os.ReadFile("../testdata/gamelists/neogeo.xml")
	if err != nil {
		t.Skip()
	}
	dir := t.TempDir()
	gl := filepath.Join(dir, "gamelist.xml")
	_ = os.WriteFile(gl, b, 0o644)
	blocks := reGameBlock.FindAllString(string(b), -1)
	rel := strings.TrimPrefix(xmlUnesc(rePathTag.FindStringSubmatch(blocks[0])[1]), "./")
	if err := applyGamelistRemove(gl, rel); err != nil {
		t.Fatal(err)
	}
	baks, _ := filepath.Glob(gl + ".multirk-bak-*")
	if len(baks) != 1 {
		t.Fatalf("backups=%d", len(baks))
	}
	bak, _ := os.ReadFile(baks[0])
	if sum(bak) != sum(b) {
		t.Fatal("el backup no es idéntico al original")
	}
	now, _ := os.ReadFile(gl)
	if n, _ := countGames(string(now)); n != len(blocks)-1 {
		t.Fatalf("juegos %d", n)
	}
	if err := applyGamelistRemove(gl, "no-existe.zip"); err != nil {
		t.Fatal(err)
	}
	same, _ := os.ReadFile(gl)
	if string(same) != string(now) {
		t.Fatal("cambió el gamelist al quitar una entrada inexistente")
	}
}
