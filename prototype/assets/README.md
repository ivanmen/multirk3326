# Assets generados en PixelLab

## Pantallas y kits

| archivo | herramienta | tamaño | coste | estado |
|---|---|---|---|---|
| `screen-frame.png` | `create_ui_asset` | 384×688 | 25 gen | **marco de la app** |
| `form-controls.png` | `create_ui_asset` | 512×512 | 25 gen | slider, progreso, checkbox, radio, switch, inputs |
| `actions-nav-v2.png` | `create_ui_asset` | 512×512 | 25 gen | **combobox, botones, tabs, segmented, badge, toast, chip (sin texto)** |
| `actions-nav.png` | `create_ui_asset` | 512×512 | 25 gen | v1 descartada (texto incrustado) |
| `ui-kit.png` | `create_ui_asset` | 512×512 | 25 gen | panel SAVE de referencia |
| `icon3.png` | `create_image_pixflux` | 256×256 | 1 gen | **icono elegido** |
| `icon.png`, `icon2.png` | `create_image_pixflux` | 128×128 | 2 gen | variantes descartadas |

## Iconos (`assets/icons/`)

16 iconos de 64×64 con transparencia:

`volume`, `mute`, `brightness`, `battery`, `wifi`, `gamepad`, `save`,
`sync`, `search`, `settings`, `chevron`, `check`, `close`, `play`, `trash`,
`cloud`.

Los job IDs de cada descarga están en `scripts/download-icons-batch*.ps1`.

## Paleta

```
#0a0716  #120c26  #1b1338  #241a4a  #3b2c6e
#a8ff60  #5ce1e6  #ffc94a  #ff6bb5  #f2ecff
```

## Prompts base

**Marco de pantalla**
> pixel art mobile app interface frame for a retro handheld save sync app:
> dark indigo panels, chunky stepped borders, lime green accent on the top bar,
> cyan details, empty content areas ready for text, flat shading, no text

**Controles de formulario**
> 8-bit pixel art UI form controls sheet: horizontal slider, progress bar,
> checkbox checked and unchecked, radio button on and off, toggle switch on and
> off, text input, search field. Dark indigo surfaces, lime green fills, cyan
> details, thick black outlines, flat shading, no text

**Acciones y navegación**
> 8-bit pixel art UI component sheet: dropdown combobox, primary/secondary/
> danger/disabled buttons, icon button, tabs, segmented control, badge, toast,
> chip. No text, no letters, no labels.

**Icono**
> pixel art app icon, exactly two identical deep purple handheld game consoles
> facing each other, bright lime green sync arrows, dark indigo background,
> cyan screens, thick black outline, flat 8-bit shading, no text

## Regenerar

```bash
# las imágenes están en PixelLab; los scripts solo descargan por job id
powershell -File scripts/download-icons-batch1.ps1
```

## Nota

`create_ui_asset` genera arte de componentes, no una pantalla compuesta ni
estados hover/pressed. La composición está en `prototype/pixellab.html`
(HTML) y se trasladará a Compose con el marco como fondo escalable.
