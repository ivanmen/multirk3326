package main

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sum(b []byte) [32]byte { return sha256.Sum256(b) }

// Real gamelists pulled from the consoles (different tools, different styles).
func TestPatchRealGamelists(t *testing.T) {
	files, _ := filepath.Glob("../testdata/gamelists/*.xml")
	if len(files) == 0 {
		t.Skip("sin gamelists de prueba")
	}
	for _, f := range files {
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			orig, _ := os.ReadFile(f)
			src := string(orig)
			blocks := reGameBlock.FindAllString(src, -1)
			if len(blocks) == 0 {
				t.Skip("sin juegos")
			}
			first := rePathTag.FindStringSubmatch(blocks[0])[1]
			rel := strings.TrimPrefix(xmlUnesc(first), "./")

			fields := gameFields{
				Desc: "Descripción de prueba con <símbolos> & \"comillas\".", Developer: "Dev",
				Publisher: "Pub", Genre: "Acción", Players: "1-2",
				Releasedate: "20010917T000000", Rating: "0.80", Image: "./media/images/x.png",
			}

			// 1) patch an existing game: only its block may change
			out, added, err := patchGamelistText(src, rel, fields)
			if err != nil || added {
				t.Fatalf("existente: err=%v added=%v", err, added)
			}
			if strings.Replace(out, "", "", 0) == src {
				t.Fatal("no cambió nada")
			}
			outBlocks := reGameBlock.FindAllString(out, -1)
			if len(outBlocks) != len(blocks) {
				t.Fatalf("juegos %d -> %d", len(blocks), len(outBlocks))
			}
			for i := range blocks {
				if i == 0 {
					continue
				}
				if blocks[i] != outBlocks[i] {
					t.Fatalf("el bloque %d cambió y no debía", i)
				}
			}
			// bytes outside the first block identical
			i0 := strings.Index(src, blocks[0])
			o0 := strings.Index(out, outBlocks[0])
			if src[:i0] != out[:o0] || src[i0+len(blocks[0]):] != out[o0+len(outBlocks[0]):] {
				t.Fatal("cambió texto fuera del bloque objetivo")
			}
			for _, keep := range []string{"playcount", "lastplayed", "favorite", "gametime"} {
				if strings.Contains(blocks[0], "<"+keep+">") && !strings.Contains(outBlocks[0], "<"+keep+">") {
					t.Fatalf("se perdió <%s>", keep)
				}
			}
			if !strings.Contains(outBlocks[0], "&lt;símbolos&gt; &amp;") {
				t.Fatal("no escapó el XML")
			}
			n, err := countGames(out)
			if err != nil || n != len(blocks) {
				t.Fatalf("XML inválido o conteo distinto: %v %d", err, n)
			}

			// 2) idempotente
			out2, _, _ := patchGamelistText(out, rel, fields)
			if out2 != out {
				t.Fatal("no es idempotente")
			}

			// 3) juego nuevo: se añade al final sin tocar lo demás
			out3, added3, err := patchGamelistText(src, "nuevo juego (test).zip", gameFields{Name: "Nuevo", Desc: "d"})
			if err != nil || !added3 {
				t.Fatalf("nuevo: %v %v", err, added3)
			}
			if !strings.HasPrefix(out3, strings.TrimRight(src[:strings.LastIndex(src, "</gameList>")], " \t")) {
				t.Fatal("el añadido alteró el contenido previo")
			}
			if n3, err := countGames(out3); err != nil || n3 != len(blocks)+1 {
				t.Fatalf("conteo tras añadir: %v %d", err, n3)
			}
		})
	}
}

// Backup -> write -> restore -> compare, as done by hand on the console.
func TestBackupRestoreCompare(t *testing.T) {
	files, _ := filepath.Glob("../testdata/gamelists/*.xml")
	for _, f := range files {
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			orig, _ := os.ReadFile(f)
			blocks := reGameBlock.FindAllString(string(orig), -1)
			if len(blocks) == 0 {
				t.Skip()
			}
			dir := t.TempDir()
			gl := filepath.Join(dir, "gamelist.xml")
			_ = os.WriteFile(gl, orig, 0o644)
			rel := strings.TrimPrefix(xmlUnesc(rePathTag.FindStringSubmatch(blocks[0])[1]), "./")

			if err := applyGamelistPatch(gl, rel, gameFields{Desc: "nueva", Developer: "X"}); err != nil {
				t.Fatal(err)
			}
			changed, _ := os.ReadFile(gl)
			if sum(changed) == sum(orig) {
				t.Fatal("no cambió")
			}
			baks, _ := filepath.Glob(gl + ".multirk-bak-*")
			if len(baks) != 1 {
				t.Fatalf("backups=%d", len(baks))
			}
			bak, _ := os.ReadFile(baks[0])
			if sum(bak) != sum(orig) {
				t.Fatal("el backup no es idéntico al original")
			}
			// restore and compare
			_ = os.WriteFile(gl, bak, 0o644)
			restored, _ := os.ReadFile(gl)
			if sum(restored) != sum(orig) {
				t.Fatal("restaurado distinto al original")
			}
			if left, _ := filepath.Glob(gl + ".multirk-tmp"); len(left) != 0 {
				t.Fatal("quedó un temporal")
			}
		})
	}
}

func TestImageConvention(t *testing.T) {
	cases := map[string][2]string{
		"neogeo.xml":    {"images", "-image"},
		"cps1.xml":      {"media/images", ""},
		"dreamcast.xml": {"downloaded_images", ""},
	}
	for f, want := range cases {
		b, err := os.ReadFile("../testdata/gamelists/" + f)
		if err != nil {
			continue
		}
		d, s := imageConvention(string(b))
		if d != want[0] || s != want[1] {
			t.Errorf("%s: got (%s,%q) want (%s,%q)", f, d, s, want[0], want[1])
		}
	}
}
