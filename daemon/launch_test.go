package main

import (
	"strings"
	"testing"
)

func TestBuildLaunchCommand(t *testing.T) {
	cfg := `<systemList><system><name>neogeo</name>
<command>sudo perfmax %GOVERNOR% %ROM%; /usr/local/bin/%EMULATOR% -L /c/%CORE%_libretro.so %ROM%; sudo perfnorm</command>
<emulators><emulator name="retroarch"><cores><core>fbneo</core><core>x</core></cores></emulator></emulators>
</system></systemList>`
	got, err := buildLaunchCommand([]byte(cfg), "neogeo", "/roms/neogeo/it's.zip")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "retroarch -L /c/fbneo_libretro.so '/roms/neogeo/it'\\''s.zip'") {
		t.Fatalf("comando inesperado: %s", got)
	}
	if _, err := buildLaunchCommand([]byte(cfg), "nope", "/x"); err == nil {
		t.Fatal("esperaba error")
	}
}
