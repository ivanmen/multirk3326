# MULTIRK

App Android y agente Go para consolas portátiles RK3326 con ArkOS. Desde el
móvil, por WiFi: navegar el catálogo, lanzar juegos, scrapear, editar y
eliminar, copiar partidas entre dos consolas y controlar volumen y brillo.

```
android/    app Compose (org.multirk.app)
daemon/     agente Go (multirkd) y CLI (multirkctl)
scripts/    compilar, instalar y desplegar
docs/       estilo de la UI y pruebas en el móvil
testdata/   ROMs y gamelists de prueba
AGENTS.md   reglas de trabajo y de producto
```

## Cómo funciona

- **Descubrimiento:** UDP multicast `239.255.42.99:47820`.
- **Datos:** TCP `47820`, frames binarios (`daemon/proto.go`).
- **Emparejado:** automático, sin PIN. El agente se instala por SSH desde la app.
- **Lanzar juegos:** el agente cierra EmulationStation, ejecuta el comando de
  `es_systems.cfg` y lo reabre al terminar.

## Compilar

```powershell
powershell -File scripts\build-android.ps1 assembleDebug   # APK
powershell -File scripts\build-agent-assets.ps1            # agente arm64/armv7
```

```bash
cd daemon && go test .
```

## Pendiente

- Sincronización de ROMs y media entre dos consolas (hoy solo las opciones).
- Probar «Jugar» en más sistemas (solo verificado con neogeo).
