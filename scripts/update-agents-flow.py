path = r"D:\source\multirk3326\AGENTS.md"
src = open(path, encoding="utf-8").read()

idx = src.index("**Continuar** exige")
start = src.rfind("\n", 0, idx) + 1
end_marker = "nunca lo leas ni escribas en `SharedPreferences`."
end = src.index(end_marker) + len(end_marker)

new_block = """3. **Catálogo** — al elegir consola(s) se entra **directo al catálogo** de la
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
(`UiState.consoleMode`); nunca lo leas ni escribas en `SharedPreferences`."""

src = src[:start] + new_block + src[end:]
open(path, "w", encoding="utf-8").write(src)
print("ok")
