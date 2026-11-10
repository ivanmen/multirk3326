Add-Type -AssemblyName System.Drawing

$root = Split-Path -Parent $PSScriptRoot
$assets = Join-Path $root "prototype\assets"
$slices = Join-Path $assets "slices"
$res = Join-Path $root "android\app\src\main\res\drawable-nodpi"
$mipmaps = @{
  "mipmap-mdpi"    = 48
  "mipmap-hdpi"    = 72
  "mipmap-xhdpi"   = 96
  "mipmap-xxhdpi"  = 144
  "mipmap-xxxhdpi" = 192
}
New-Item -ItemType Directory -Force -Path $slices, $res | Out-Null
Get-ChildItem $res -Filter "*.9.png" -ErrorAction SilentlyContinue | Remove-Item -Force

function Recolor([string]$dir, [string]$name, [int]$rMin, [int]$gMin, [int]$bMax, [int]$toR, [int]$toG, [int]$toB) {
  $p = Join-Path $dir "$name.png"
  $src = [System.Drawing.Bitmap]::FromFile($p)
  $bmp = New-Object System.Drawing.Bitmap $src.Width, $src.Height
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.DrawImage($src, 0, 0)
  $g.Dispose(); $src.Dispose()
  for ($y = 0; $y -lt $bmp.Height; $y++) {
    for ($x = 0; $x -lt $bmp.Width; $x++) {
      $c = $bmp.GetPixel($x, $y)
      if ($c.A -gt 0 -and $c.R -ge $rMin -and $c.G -ge $gMin -and $c.B -le $bMax) {
        $bmp.SetPixel($x, $y, [System.Drawing.Color]::FromArgb($c.A, $toR, $toG, $toB))
      }
    }
  }
  $bmp.Save($p, [System.Drawing.Imaging.ImageFormat]::Png)
  $bmp.Save((Join-Path $slices "$name.png"), [System.Drawing.Imaging.ImageFormat]::Png)
  $bmp.Dispose()
  Write-Host "recolor $name"
}

function Crop([string]$file, [int]$x, [int]$y, [int]$w, [int]$h, [string]$name) {
  $img = [System.Drawing.Bitmap]::FromFile($file)
  $bmp = New-Object System.Drawing.Bitmap $w, $h
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::Half
  $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::NearestNeighbor
  $g.DrawImage($img, (New-Object System.Drawing.Rectangle 0, 0, $w, $h),
               (New-Object System.Drawing.Rectangle $x, $y, $w, $h),
               [System.Drawing.GraphicsUnit]::Pixel)
  $bmp.Save((Join-Path $slices "$name.png"), [System.Drawing.Imaging.ImageFormat]::Png)
  $bmp.Save((Join-Path $res "$name.png"), [System.Drawing.Imaging.ImageFormat]::Png)
  $g.Dispose(); $bmp.Dispose(); $img.Dispose()
  Write-Host "slice $name"
}

function NinePatch([string]$name) {
  $srcFile = Join-Path $res "$name.png"
  $img = [System.Drawing.Bitmap]::FromFile($srcFile)
  $w = $img.Width; $h = $img.Height
  $bmp = New-Object System.Drawing.Bitmap ($w + 2), ($h + 2)
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.Clear([System.Drawing.Color]::Transparent)
  $g.DrawImage($img, 1, 1, $w, $h)
  $sx1 = [Math]::Max(1, [int]($w * 0.33)); $sx2 = [Math]::Min($w - 2, [int]($w * 0.67))
  $sy1 = [Math]::Max(1, [int]($h * 0.33)); $sy2 = [Math]::Min($h - 2, [int]($h * 0.67))
  for ($x = $sx1; $x -le $sx2; $x++) { $bmp.SetPixel($x + 1, 0, [System.Drawing.Color]::Black) }
  for ($y = $sy1; $y -le $sy2; $y++) { $bmp.SetPixel(0, $y + 1, [System.Drawing.Color]::Black) }
  $bmp.Save((Join-Path $res "nine_$name.9.png"), [System.Drawing.Imaging.ImageFormat]::Png)
  $g.Dispose(); $bmp.Dispose(); $img.Dispose()
  Write-Host "9patch nine_$name.9.png"
}

# ---- form controls sheet (512x512) ----
$form = Join-Path $assets "form-controls.png"
Crop $form 18 22 238 44 "ui_slider"
Crop $form 266 28 212 32 "ui_progress"
Crop $form 18 106 42 42 "ui_checkbox_off"
Crop $form 70 106 42 42 "ui_checkbox_on"
Crop $form 298 105 44 44 "ui_radio_off"
Crop $form 360 105 44 44 "ui_radio_on"
Crop $form 18 184 74 42 "ui_switch_off"
Crop $form 104 184 74 42 "ui_switch_on"
Crop $form 18 264 232 52 "ui_input"
Crop $form 266 264 232 52 "ui_search"

# ---- actions/nav sheet v2 (512x512) ----
$act = Join-Path $assets "actions-nav-v2.png"
Crop $act 18 18 224 54 "ui_combo"
Crop $act 18 76 232 152 "ui_combo_open"
Crop $act 274 80 212 62 "ui_btn_primary"
Crop $act 274 142 212 62 "ui_btn_secondary"
Crop $act 274 18 212 62 "ui_btn_danger"
Recolor $res "ui_btn_danger" 180 120 130 255 93 93
Crop $act 274 204 212 62 "ui_btn_disabled"
Crop $act 274 274 62 62 "ui_icon_btn"
Crop $act 18 244 102 52 "ui_tab_on"
Crop $act 118 244 102 52 "ui_tab_off"
Crop $act 18 306 232 72 "ui_tabbar"
Crop $act 18 386 232 56 "ui_segmented"
Crop $act 274 338 82 38 "ui_badge"
Crop $act 274 384 212 56 "ui_toast"
Crop $act 274 444 112 44 "ui_chip"

# ---- dialogs / cards sheet (512x512) ----
$dia = Join-Path $assets "dialogs-cards.png"
Crop $dia 20 20 308 208 "ui_dialog"
Crop $dia 26 26 292 40 "ui_dialog_header"
Crop $dia 36 172 128 44 "ui_dialog_btn1"
Crop $dia 176 172 128 44 "ui_dialog_btn2"
Crop $dia 18 244 232 132 "ui_card"
Crop $dia 18 384 232 54 "ui_list_row"
Crop $dia 294 254 192 66 "ui_tooltip"
Crop $dia 294 338 192 122 "ui_popover"
Crop $dia 294 454 192 52 "ui_empty"

# 9-patch only for stretchable rectangles
foreach ($n in @("ui_btn_primary","ui_btn_secondary","ui_btn_danger","ui_btn_disabled",
                 "ui_input","ui_search","ui_combo","ui_toast","ui_badge","ui_tab_on","ui_tab_off",
                 "ui_dialog","ui_card","ui_popover","ui_tooltip")) {
  NinePatch $n
}

# ---- full screen frame ----
Copy-Item (Join-Path $assets "screen-frame.png") (Join-Path $res "ui_screen_frame.png") -Force

# ---- icons ----
$iconDir = Join-Path $assets "icons"
Get-ChildItem $iconDir -Filter *.png | ForEach-Object {
  Copy-Item $_.FullName (Join-Path $res ("ic_" + $_.Name)) -Force
}
Write-Host "icons copied"

# ---- launcher icon in all densities ----
$srcIcon = [System.Drawing.Bitmap]::FromFile((Join-Path $assets "icon3.png"))
foreach ($d in $mipmaps.Keys) {
  $size = $mipmaps[$d]
  $dir = Join-Path $root ("android\app\src\main\res\" + $d)
  New-Item -ItemType Directory -Force -Path $dir | Out-Null
  $bmp = New-Object System.Drawing.Bitmap $size, $size
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::NearestNeighbor
  $g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::Half
  $g.DrawImage($srcIcon, 0, 0, $size, $size)
  $bmp.Save((Join-Path $dir "ic_launcher.png"), [System.Drawing.Imaging.ImageFormat]::Png)
  $g.Dispose(); $bmp.Dispose()
}
$srcIcon.Dispose()
Write-Host "launcher icons done"

# ---- fonts ----
$fontDir = Join-Path $root "android\app\src\main\res\font"
New-Item -ItemType Directory -Force -Path $fontDir | Out-Null
$fonts = @{
  "press_start_2p.ttf" = "https://github.com/google/fonts/raw/main/apache/pressstart2p/PressStart2P-Regular.ttf"
  "vt323.ttf"          = "https://github.com/google/fonts/raw/main/ofl/vt323/VT323-Regular.ttf"
}
foreach ($name in $fonts.Keys) {
  $dest = Join-Path $fontDir $name
  if (-not (Test-Path $dest)) {
    Invoke-WebRequest -Uri $fonts[$name] -OutFile $dest -UseBasicParsing
    Write-Host "font $name"
  }
}

Write-Host "DONE"
