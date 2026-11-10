package main

import (
	"os"
	"strings"
	"testing"
)

func TestPatchSetNameOnlyWhenAsked(t *testing.T) {
	b, err := os.ReadFile("../testdata/gamelists/ports.xml")
	if err != nil {
		t.Skip()
	}
	src := string(b)
	blocks := reGameBlock.FindAllString(src, -1)
	rel := strings.TrimPrefix(xmlUnesc(rePathTag.FindStringSubmatch(blocks[0])[1]), "./")

	keep, _, _ := patchGamelistText(src, rel, gameFields{Name: "Otro", Desc: "x"})
	if strings.Contains(keep, "<name>Otro</name>") {
		t.Fatal("cambió el nombre sin SetName")
	}
	set, _, err := patchGamelistText(src, rel, gameFields{Name: "Mi nombre", SetName: true, Desc: "x"})
	if err != nil || !strings.Contains(set, "<name>Mi nombre</name>") {
		t.Fatalf("no cambió el nombre con SetName: %v", err)
	}
	if n, err := countGames(set); err != nil || n != len(blocks) {
		t.Fatalf("conteo %d err=%v", n, err)
	}
	for _, kept := range []string{"playcount", "lastplayed"} {
		if strings.Contains(blocks[0], "<"+kept+">") && !strings.Contains(set, "<"+kept+">") {
			t.Fatalf("se perdió %s", kept)
		}
	}
}
