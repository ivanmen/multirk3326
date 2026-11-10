Add-Type -AssemblyName System.Drawing
$dest = "android\app\src\main\res\font\press_start_2p.ttf"
Invoke-WebRequest -Uri "https://fonts.gstatic.com/s/pressstart2p/v16/e3t4euO8T-267oIAQAu6jDQyK3nVivY.ttf" -OutFile $dest -UseBasicParsing -TimeoutSec 30
$b = [System.IO.File]::ReadAllBytes($dest)
Write-Host ("sig: " + (($b[0..7] | ForEach-Object { $_.ToString("X2") }) -join " ") + "  size=$($b.Length)")
foreach ($f in @($dest, "android\app\src\main\res\font\vt323.ttf")) {
  try {
    $fc = New-Object System.Drawing.Text.PrivateFontCollection
    $fc.AddFontFile((Resolve-Path $f))
    Write-Host "$f -> $($fc.Families[0].Name)"
  } catch {
    Write-Host "$f -> BAD"
  }
}
