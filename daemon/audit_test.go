package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const auditGL = `<?xml version="1.0"?>
<gameList>
	<game>
		<path>./Existe Uno.zip</path>
		<name>Existe Uno</name>
		<playcount>3</playcount>
	</game>
	<game>
		<path>./Falta &amp; Co.zip</path>
		<name>Falta</name>
	</game>
	<game>
		<path>./sub/Existe Dos.zip</path>
		<name>Existe Dos</name>
	</game>
	<game>
		<path>./sub/Falta Dos.zip</path>
		<name>Falta Dos</name>
	</game>
	<game>
		<path>../fuera.zip</path>
		<name>Fuera (no se toca)</name>
	</game>
</gameList>
`

func TestAuditGamelistFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "Existe Uno.zip"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "sub", "Existe Dos.zip"), []byte("x"), 0o644)
	gl := filepath.Join(dir, "gamelist.xml")
	_ = os.WriteFile(gl, []byte(auditGL), 0o644)

	checked, removed, err := auditGamelistFile(gl, false)
	if err != nil {
		t.Fatal(err)
	}
	if checked != 4 {
		t.Fatalf("revisadas=%d, esperaba 4 (la de ../ no cuenta)", checked)
	}
	if len(removed) != 2 {
		t.Fatalf("quitadas=%v", removed)
	}
	out, _ := os.ReadFile(gl)
	s := string(out)
	for _, keep := range []string{"Existe Uno.zip", "Existe Dos.zip", "../fuera.zip", "<playcount>3</playcount>"} {
		if !strings.Contains(s, keep) {
			t.Fatalf("se perdió %q", keep)
		}
	}
	for _, gone := range []string{"Falta &amp; Co.zip", "Falta Dos.zip"} {
		if strings.Contains(s, gone) {
			t.Fatalf("sigue %q", gone)
		}
	}
	if n, err := countGames(s); err != nil || n != 3 {
		t.Fatalf("juegos=%d err=%v", n, err)
	}
	baks, _ := filepath.Glob(gl + ".multirk-bak-*")
	if len(baks) != 1 {
		t.Fatalf("backups=%d", len(baks))
	}
	if b, _ := os.ReadFile(baks[0]); string(b) != auditGL {
		t.Fatal("el backup no es idéntico al original")
	}

	// segunda pasada: ya está limpio, no cambia nada y no crea otro backup
	_, removed2, err := auditGamelistFile(gl, false)
	if err != nil || len(removed2) != 0 {
		t.Fatalf("2a pasada: %v %v", removed2, err)
	}
	if b2, _ := filepath.Glob(gl + ".multirk-bak-*"); len(b2) != 1 {
		t.Fatal("se creó un backup innecesario")
	}
}

func TestAuditRealGamelistsNoOpWhenFilesExist(t *testing.T) {
	// con los gamelists reales de prueba y ninguna ROM en disco, todo es "faltante":
	// comprueba que el recuento y la verificación XML aguantan archivos grandes.
	b, err := os.ReadFile("../testdata/gamelists/neogeo.xml")
	if err != nil {
		t.Skip()
	}
	dir := t.TempDir()
	gl := filepath.Join(dir, "gamelist.xml")
	_ = os.WriteFile(gl, b, 0o644)
	total := len(reGameBlock.FindAllString(string(b), -1))
	checked, removed, err := auditGamelistFile(gl, false)
	if err != nil {
		t.Fatal(err)
	}
	if checked != total || len(removed) != total {
		t.Fatalf("checked=%d removed=%d total=%d", checked, len(removed), total)
	}
	out, _ := os.ReadFile(gl)
	if n, err := countGames(string(out)); err != nil || n != 0 {
		t.Fatalf("quedan %d juegos (err=%v)", n, err)
	}
}
