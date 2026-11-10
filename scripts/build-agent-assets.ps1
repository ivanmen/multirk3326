$root = Split-Path -Parent $PSScriptRoot
$daemon = Join-Path $root "daemon"
$assets = Join-Path $root "android\app\src\main\assets"
New-Item -ItemType Directory -Force -Path $assets | Out-Null

Push-Location $daemon
$env:CGO_ENABLED = "0"

$env:GOOS = "linux"; $env:GOARCH = "arm64"
go build -trimpath -ldflags "-s -w" -o "dist\multirkd-linux-arm64" .
if ($LASTEXITCODE -ne 0) { Pop-Location; exit 1 }

$env:GOOS = "linux"; $env:GOARCH = "arm"; $env:GOARM = "7"
go build -trimpath -ldflags "-s -w" -o "dist\multirkd-linux-armv7" .
if ($LASTEXITCODE -ne 0) { Pop-Location; exit 1 }

Pop-Location
Copy-Item (Join-Path $daemon "dist\multirkd-linux-arm64") (Join-Path $assets "multirkd-linux-arm64") -Force
Copy-Item (Join-Path $daemon "dist\multirkd-linux-armv7") (Join-Path $assets "multirkd-linux-armv7") -Force

Get-ChildItem $assets | Select-Object Name, Length
