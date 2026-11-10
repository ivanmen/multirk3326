# AGENTS.md — MULTIRK

Reglas de trabajo para agentes que toquen este repositorio. Léelas antes de
cambiar UI, protocolo o scripts de build.

## Dirección visual — “cozy” (en migración)

La UI se está rehaciendo: el pixel-art oscuro se sustituye por el estilo
**cozy** de `docs/UI_STYLE.md` (fuente de verdad del arte).

- Paleta: fondo `#F2E5CC`, tinta `#3A2A1C`, tarjeta `#FFF8EC`, primario
  `#C8622F`, secundario `#6F8F72`.
- Tipografía **Baloo 2** (`res/font/baloo2_*`): títulos ExtraBold, cuerpo
  SemiBold. Nada de MAYÚSCULAS pixel en pantallas nuevas.
- Botones redondeados (13 dp) con **borde inferior sólido de 5 dp**; tarjetas
  de 18 dp con borde 2 dp y sombra suave de 4 dp.
- Componentes nuevos en `ui/Cozy.kt`. No añadas pixel-art nuevo; las pantallas
  que aún no se migren conservan el tema anterior.
- Iconos: `res/drawable-nodpi/cozy_*.png` (PixelLab, 64×64, fondo transparente,
  paleta cozy). No uses emoji ni los iconos neón antiguos en pantallas nuevas.
- El flujo de arranque tiene dos pasos previos (ver “Flujo de arranque”). El
  modo **no se guarda**: se pregunta en cada apertura, **antes** del historial.

## Flujo de arranque (en cada apertura)

El **orden es fijo**: primero el modo, después el historial (que depende del
modo), después la app.

1. **`StartScreen`** (pantalla de inicio) — pregunta **¿1 o 2 consolas?**
2. **`HistoryScreen`** — historial de conexiones, adaptado al modo elegido:
   título “Elige tu consola” (modo 1) o “Elige tus dos consolas” (modo 2),
   con IP y última vez visto, y el botón **Añadir consola nueva**.
   **No hay botón Continuar**: se avanza tocando el panel de la consola.
   - **Modo 1**: tocar una consola activa continúa al instante.
   - **Modo 2**: el 1.er toque = ORIGEN, el 2.º = DESTINO (etiqueta en cada
     panel). Tocar una seleccionada **deshace** la selección. Con las dos
     elegidas la app avanza sola tras ~1 s; tocar una antes lo cancela.
   - El botón **Info** de cada panel (consolas emparejadas) abre su ficha.
   - Tocar una consola activa **sin emparejar** abre el emparejado.
   - Se puede **arrastrar hacia abajo** (pull-to-refresh) para volver a buscar.
   Al abrir lanza el descubrimiento multicast (y lo repite cada 30 s). Una
   consola que **no responde** se muestra **Inactiva**: atenuada, sin poder
   tocarla. Mientras se busca, todas dicen “Buscando…”. Una consola
   **activa** tiene el marco verde salvia que **brilla y late** (borde 2→3 dp
   y sombra de color, ciclo de 1,1 s); la inactiva no se anima.
3. **Catálogo** — al elegir consola(s) se entra **directo al catálogo** de la
   consola ORIGEN. **No existe la pantalla “Consolas”** (`Screen.Devices` se
   eliminó): no la vuelvas a crear. Desde aquí se accede a Controles y Ajustes
   por la barra inferior.

### Pantallas y navegación atrás

| Pantalla (`Screen`) | Atrás lleva a |
|---|---|
| `Start` (¿1 o 2 consolas?) | cierra la app |
| `History` (Elige tu consola) | `Start` |
| `Catalog` | `History` |
| `Game` | `Catalog` |
| `Controls` | `Catalog` |
| `Settings` | `Catalog` |

Cada pantalla registra su `BackHandler`; no dependas del comportamiento por
defecto. Los diálogos se cierran con atrás antes de navegar.

Barra inferior: **Catálogo, Controles, Ajustes** (en ambos modos). En modo 2 el
catálogo muestra “ORIGEN → DESTINO” y un botón **⇄** que intercambia roles y
abre el catálogo de la otra consola.

Mientras no se haya elegido el modo **no hay barra inferior ni acceso a
Catálogo, Controles o Ajustes**. El modo vive solo en memoria
(`UiState.consoleMode`); nunca lo leas ni escribas en `SharedPreferences`.

## Descubrimiento de consolas (regla técnica)

- El demonio contesta un `probe` con un `announce` **unicast al origen del
  probe** (IP y puerto). Por eso el cliente **debe enviar el probe desde el
  mismo socket que escucha** (`MulticastSocket` en 47820). Enviarlo desde un
  socket temporal pierde la respuesta (bug que hizo parecer inactivas a
  consolas encendidas).
- Además del multicast, el cliente manda el probe **unicast a cada consola
  conocida**: el multicast por WiFi es poco fiable. Reenvía hasta 4 veces
  (cada 400 ms) y el timeout total es 1,5 s.
- Cada consola se marca activa **en cuanto responde** (callback `onFound`), no
  al final. Mientras dura la búsqueda las demás dicen “Buscando…”.
- Medidas de referencia: el demonio responde en ~10 ms; un escaneo con una
  consola apagada dura ~1,5 s (espera al timeout). Con todas encendidas termina
  en ~0,7 s.
- La traza `multirk-scan` (logcat) da el tiempo de cada escaneo.

## Ficha de consola (botón Info) y desemparejar

- La ficha solo tiene **Desemparejar** y **Cerrar** (y **Emparejar** si la
  consola no está emparejada). No hay “Abrir catálogo” ni “Quitar agente”.
- **Desemparejar borra el agente y desempareja** en un solo paso: abre el
  diálogo “Desemparejar consola” (IP, usuario y contraseña SSH), desinstala el
  agente por SSH con purga de datos, y después olvida la consola
  (`AgentState.mode == "unpair"`).
- Si la consola no responde por SSH, el diálogo ofrece **Desemparejar sin
  borrar agente** como salida.
- Nunca ejecutes el borrado del agente en las pruebas contra consolas reales:
  prueba hasta el diálogo y pulsa **Cancelar**.

## Dos modos de uso

El modo se puede cambiar en **Ajustes** durante la sesión.

- **Una consola** (`consoleMode = 1`): solo catálogo y controles. No hay
  destino, sincronización ni segunda consola; se empareja como máximo una.
- **Dos consolas** (`consoleMode = 2`): copiar partidas entre ORIGEN y DESTINO
  (más adelante también ROMs). Requiere las dos consolas emparejadas.

La barra inferior es la misma en ambos modos (Catálogo, Controles, Ajustes).
En modo 1 el destino es siempre `null` (`targetFor`), aunque haya otra consola
emparejada.

Nunca se salta sola al catálogo al abrir la app ni da por hecho que hay una
consola conectada: la elección siempre pasa por `Start` e `History`.

## REGLA #1 — Botones y tipografía (error recurrente)

Este error ya se ha repetido varias veces: **botones diminutos dentro de
diálogos**. No vuelvas a hacerlo.

### Botones en diálogos

- **Todo botón de un diálogo usa `large = true`.** Sin excepciones.
- Nunca uses `small = true` en un diálogo (`Dialog { ... }`).
- Nunca uses el tamaño por defecto (`8sp`) para acciones de diálogo.
- Los botones van **apilados en una `Column`**, uno por fila, a ancho completo:

```kotlin
Spacer(Modifier.height(10.dp))
Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
    PixelButton("QUITAR AGENTE", { ... }, large = true, color = Panel2, textColor = Danger)
    PixelButton("DESEMPAREJAR", { ... }, large = true, color = Danger, textColor = Ink)
    PixelButton("CERRAR", { ... }, large = true, color = Panel2, textColor = TextMain)
}
```

- **Prohibido** poner dos botones `large` en una `Row` con `Modifier.weight(1f)`:
  el texto pixel se parte en dos líneas. Si hacen falta dos acciones, se apilan.
- Orden dentro del diálogo: acción destructiva primero, luego secundaria,
  **CERRAR siempre al final**.

### Tamaños disponibles en `PixelButton` (`ui/Theme.kt`)

| Variante          | Padding vertical | Fuente | Uso permitido                                    |
|-------------------|------------------|--------|--------------------------------------------------|
| `small = true`    | 6 dp             | 7 sp   | Solo acciones auxiliares dentro de una fila de lista (p. ej. ENVIAR, EMPAREJAR en una fila) |
| por defecto       | 11 dp            | 8 sp   | CTA de pantalla completa, acciones de tarjeta     |
| `large = true`    | 18 dp            | 13 sp  | **Todos los botones de diálogo** y CTAs principales |

Nunca inventes padding ni tamaños de fuente fuera de estas tres variantes.
Si un botón parece pequeño, sube de variante; no añadas `fontSize` a mano.

### Etiquetas

- Las etiquetas de botón van en **UNA sola línea**. Si no cabe, **acorta el
  texto** (`DESINSTALAR AGENTE` → `QUITAR AGENTE`), nunca reduzcas la fuente.
- Texto en MAYÚSCULAS, sin emoji, sin signos decorativos.

### Texto en diálogos

- Títulos: `PxText` de **9–11 sp**.
- Cuerpo y valores: `BodyText` de **15 sp como mínimo** (`15`–`17`).
- Errores y avisos: `BodyText` **15 sp**.
- Logs: `BodyText` **14 sp**.
- No uses `BodyText` por debajo de 13 sp salvo en metadatos de filas de lista.

### Densidad y fuentes

- `MultirkTheme` aplica **1.25× de densidad global**. No la dupliques ni la
  compensas con tamaños menores.
- Fuentes fijas: `PixelFont` (Press Start 2P) para títulos/etiquetas y
  `BodyFont` (VT323) para cuerpo. No añadas tipografías nuevas.

### Checklist obligatorio antes de dar por terminado un cambio de UI

1. `grep -n "small = true"` en `android/app/src/main/java/org/multirk/app/ui/`
   y justifica cada aparición: debe estar dentro de una fila de lista, nunca
   en un `Dialog`.
2. Revisa que ningún botón nuevo quede por debajo de `large` si está en un diálogo.
3. Compila: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build-android.ps1 assembleDebug`.
4. Instala y haz captura real en el teléfono (ADB por WiFi, ver “Verificación”) antes de
   afirmar que el estilo es correcto. Sin captura, no está verificado.

## Arquitectura

- `daemon/` — agente Go para RK3326/ArkOS. Protocolo binario TCP + descubrimiento
  UDP multicast. systemd, sin dependencias externas.
- `android/` — app Compose. Habla con el daemon por TCP; SSH (JSch) solo para
  instalar/desinstalar el agente.
- `api/`, `prototype/`, `tools/`, `testdata/` — apoyo y prototipos.

### Protocolo

- Descubrimiento: UDP multicast `239.255.42.99:47820`.
- Datos: TCP `47820`, frames binarios. HTTP solo depuración.
- El catálogo **nunca** se carga entero en la UI: llega comprimido (gzip) una
  vez, se guarda en SQLite y se pagina de 6 en 6, 2 columnas.
- **Controles y Ajustes**: Controles actúa **siempre sobre la consola elegida al
  inicio** (el origen, `UiState.selected`); no hay selector de consola. Ajustes
  tiene **solo** “Borrar caché local” (catálogos + portadas del móvil); no
  añadas ahí modo, consolas ni cuentas. El volumen usa `amixer` **sin `-M`**: en
  ArkOS (rk817) el mapeo por dB es absurdo y solo daba 0 % o 100 %.
- **Consola desconectada**: cualquier fallo de conexión con la consola (no un
  error que ella responda) abre una ventana bloqueante “Consola desconectada”
  con **Reconectar** y **Cerrar app**. Si al reconectar la consola aún no
  responde, la ventana sigue, avisa y el botón queda libre para reintentar.
  Usa `ConsoleException` (error respondido) vs `isConnectionLoss()`; no añadas
  polling para detectarlo: se detecta al fallar una operación.
- **Actualizar el catálogo**: solo arrastrando hacia abajo (pull-to-refresh) y
  con diálogo **Actualizar / Cancelar**. Sin botón ↻ ni mensajes de caché.
  El alcance depende del filtro: con **Todos** la alerta dice “TODOS los
  sistemas” (la consola relee toda la SD y se baja el catálogo completo); con
  un **sistema elegido** dice “solo «X»” (`REFRESH` con `system` + `CATALOG`
  con `system`: la consola relee solo esa carpeta y el móvil reemplaza solo
  ese sistema). Scrapear/editar/eliminar también reescanean solo su sistema.
- **Info manual** (sistemas sin scraper, p. ej. `ports`; el agente marca cada
  juego con `scrapable`): en vez de “Scrapear” sale **Añadir información**,
  un diálogo con imagen (selector del móvil, reducida a ≤800 px JPEG) y campos.
  Op `SETMETA` (20); mismo parche seguro del gamelist. En pruebas contra
  consolas reales **revierte siempre** desde el backup y verifica por sha256.
- **Scraping** (botón “Scrapear y actualizar” en la ficha del juego): el móvil
  solo manda la orden `SCRAPE` (op 19); la cuenta de ScreenScraper de Ajustes
  es **opcional**. Sin ella la consola usa el acceso de desarrollador del ES
  instalado, leído en tiempo de ejecución desde su binario (nunca copiado al
  código, a disco ni al móvil). **La consola** descarga datos e imagen (español, si no inglés),
  hace backup (`gamelist.xml.multirk-bak-*`, máx. 3), parchea **solo ese
  `<game>`** respetando carpeta/estilo de imagen y campos (`playcount`…) de ese
  sistema, verifica el XML, reindexa y el móvil refresca ficha, portada y
  catálogo. Si cambias `daemon/scrape.go`, corre `go test` en `daemon/`
  (usa gamelists reales de `testdata/gamelists`) antes de desplegar.
- **Sistemas ocultos** (solo en la app, nunca en la consola): mantener pulsado
  un sistema del catálogo muestra “Ocultar «X»”; Ajustes tiene un botón
  “Sistemas ocultos” que abre una **ventana** con la lista y **Mostrar** por
  sistema. Se guarda por consola en `meta.hiddenSystems`
  (semilla: tools, easyrpg, found.000, launchimages, themes, system volume
  information, indexervolumeguid, wpsettings) y “Borrar caché local” **no** lo
  borra. Siguen fijos aparte: carpetas con “backup” y entradas “Scan for new games”.
- **Dos consolas, catálogo**: con origen y destino elegidos, la pestaña
  Catálogo **no lista juegos**: muestra “Sincronizar” con dos opciones
  (**Sincronización completa** / **Por sistema**, con los sistemas del origen).
  El origen (hoy la AISLPC .37) **nunca se modifica**: ni ROMs ni media; solo
  se completa el destino (XiFan .52). Por ahora solo son opciones, no copian.
- **Jugar** (ficha del juego → “Jugar”, con alerta; ventana “Jugando” con
  **Cerrar juego**): ops `LAUNCH` (23) y `STOP` (24). El agente lanza el
  comando de `es_systems.cfg` (primer emulador/core del sistema) en una unidad
  transitoria `multirk-game` (`sudo systemd-run`), **fuera** del cgroup del
  agente (que tiene `MemoryMax`/`CPUQuota`). El script para ES, ejecuta el
  juego, hace `perfnorm` y arranca ES de nuevo, aunque el juego se cuelgue.
  El servicio del agente **no** debe llevar `NoNewPrivileges` (rompe `sudo`).
  Probado solo en la .37 con neogeo; en pruebas avisa al usuario: la pantalla
  física cambia. **Reiniciar ES es inevitable** (ES retiene `/dev/dri/card0` y
  el audio, y no tiene orden de lanzamiento externa); medido: cerrarlo sin
  cambios propios **no reescribe** los gamelists (fechas intactas tras 4
  reinicios); sí escribe unos KB (`es_settings.cfg`, `recovery/*.xml`). No se
  midió la E/S real (la consola no expone `/proc/PID/io`). No lo
  mates sin guardar ni lo pauses con señales.
- **Auditoría de gamelists** (Ajustes → “Auditar gamelists”, con alerta
  Auditar/Cancelar): op `AUDIT` (22). La consola compara **cada** `gamelist.xml`
  (raíz y subcarpetas de cada sistema; salta carpetas ocultas y de “backup”)
  con sus ROMs y quita las entradas cuyo archivo no existe, con backup +
  verificación. No toca ROMs ni media, ni rutas que apunten fuera de la
  carpeta. Después el móvil refresca solo los sistemas que cambiaron y muestra
  un informe. `multirkctl audit -dry` solo informa, sin escribir.
- **Eliminar juego** (último botón de la ficha, con alerta Eliminar/Cancelar):
  op `DELETE` (21). **Borra de verdad, sin papelera**: la consola elimina la
  ROM y su media (imagen, miniatura, marquee, vídeo; nunca el que comparta otro
  juego ni nada fuera de la carpeta del sistema), quita la entrada del gamelist
  (backup + verificación) y reindexa. El móvil limpia solo ese juego de su
  caché (fila, saves, portada, página en pantalla). Los saves no se tocan. En
  pruebas contra consolas reales usa **solo un archivo desechable**, nunca ROMs
  del usuario.
- **Leer lo mínimo de la SD de la consola.** El agente solo lee la SD cuando
  hace falta: el catálogo (gamelists) se indexa una vez y se sirve desde RAM,
  y las imágenes se leen **solo bajo demanda** (cuando el item aparece en
  pantalla). Todo lo que llega al móvil se guarda en caché (catálogo con los
  datos del gamelist en SQLite, portadas en disco) y las fichas se abren desde
  esa caché, no desde la consola. Si algo se edita o se scrapea, se
  **refresca solo esa parte** (ese juego/sistema), nunca un reescaneo total
  ni una descarga masiva.
- **Imágenes: nunca las bajes todas juntas.** Se piden solo cuando el item
  aparece en pantalla (máx. 2 a la vez), el agente las envía reducidas
  (JPEG ≤480 px) y se guardan en caché de disco para no pedirlas otra vez.
- No añadas polling: el descubrimiento es automático cada 30 s en el historial
  de conexiones; el resto se lee bajo demanda.

## Reglas de producto

- **Sin límite de consolas emparejadas**: el historial puede tener varias
  emparejadas y en línea. Tocar una sin emparejar la empareja y abre su
  catálogo; nunca muestres «máximo N consolas». En modo 2 solo se
  **seleccionan** 2 (ORIGEN y DESTINO).
- Añadir consola = instalar agente por SSH + emparejar automáticamente. El
  usuario no escribe nada.
- Un solo control para añadir consola: el icono gamepad del encabezado, con
  pulso animado (borde/sombra) que invita a tocarlo. No añadas un botón “+”
  aparte, ni “Añadir por IP”, ni “Instalar/Desinstalar” en la lista.
- El **identificador de una consola no se escribe a mano**: se toma del modelo
  de hardware que reporta el agente (`Hardware` de `/proc/cpuinfo`), por
  ejemplo `AISLPC R36TMax` o `XiFan XF40H`. Nada de nombres genéricos como
  “RG351V” por defecto. El agente lo publica en `model` (INFO y anuncio UDP)
  y la app renombra sola las consolas que aún tengan un nombre automático.
- El catálogo siempre es de **una sola consola**. El encabezado **no** lleva
  subtítulo con nombre ni IP (solo “ORIGEN → DESTINO” en modo 2).
  El catálogo se abre al elegir la consola en el historial.
- Nada de secciones técnicas visibles al usuario (“PROTOCOLO”, “UI KIT”).
- Un solo tab resaltado a la vez.

## Verificación

- **Tras cualquier cambio de UI, compila e instala siempre el APK en el
  teléfono** (`adb -s <ip>:5555 install -r`) sin esperar a que lo pidan.
- **No existe el PIN** (ni en la app ni en el agente): no lo reintroduzcas ni
  pidas códigos de autorización al usuario. El emparejado es automático.

- **ADB por WiFi es el método por defecto.** Conectar a la IP actual del
  teléfono (hoy `192.168.1.5:5555`; antes era `.14`, el router la cambió).
- Si la IP cambia: ejecutar `scripts/scan-adb-wifi.ps1` para encontrarla y
  `adb connect IP:5555`.
- El **USB solo se usa** para volver a habilitar TCP si el ADB inalámbrico se
  cae: `adb -s <serial> tcpip 5555` y después `adb connect IP:5555`.
- **Prohibido usar el emulador de Android** (`emulator-*`, AVDs) para instalar,
  probar o capturar pantallas. No forma parte del flujo y no refleja el
  dispositivo real.
- Build Android: `scripts/build-android.ps1 assembleDebug`.
- Build agente: `scripts/build-agent-assets.ps1`.
- APK: `android/app/build/outputs/apk/debug/app-debug.apk`.
- Capturas y XML de UI en `prototype/assets/screens/`.
- Hay git (rama `main`, remoto `origin`). No hagas commits ni push sin que el
  usuario lo pida. Los binarios del agente en `android/.../assets` y los scripts
  de prueba (`scripts/phone-*`…) están en `.gitignore`.
