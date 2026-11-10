$root = Split-Path -Parent $PSScriptRoot
$src = Join-Path $root "testdata\states"
$dst = Join-Path $root "testdata\states-b"
if (Test-Path $dst) { Remove-Item $dst -Recurse -Force }
Copy-Item $src $dst -Recurse -Force

$exe = Join-Path $root "daemon\dist\multirkd.exe"
$cfg = "..\testdata\config-b.json"
$work = Join-Path $root "daemon"
Start-Process -FilePath $exe -ArgumentList @("-config", $cfg) -WorkingDirectory $work -WindowStyle Hidden
Start-Sleep -Seconds 2

Write-Host "daemon B en 0.0.0.0:47834"
