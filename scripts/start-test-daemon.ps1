$root = Split-Path -Parent $PSScriptRoot
$exe = Join-Path $root "daemon\dist\multirkd.exe"
$cfg = Join-Path $root "testdata\config-auth.json"
$work = Join-Path $root "daemon"

$running = Get-Process multirkd -ErrorAction SilentlyContinue
if (-not $running) {
    Write-Host "Arrancando multirkd en 0.0.0.0:47832 ..."
    Start-Process -FilePath $exe -ArgumentList @("-config", $cfg, "-listen", "0.0.0.0:47832") -WorkingDirectory $work -WindowStyle Hidden
    Start-Sleep -Seconds 2
} else {
    Write-Host "multirkd ya esta en ejecucion (PID $($running.Id -join ','))"
}

Write-Host ""
Write-Host "IPs de este PC:"
Get-NetIPAddress -AddressFamily IPv4 |
    Where-Object { $_.IPAddress -notlike "127.*" -and $_.IPAddress -notlike "169.254.*" } |
    Select-Object IPAddress, InterfaceAlias | Format-Table -AutoSize

Write-Host "Prueba local:"
Write-Host "  daemon\dist\multirkctl.exe -addr 127.0.0.1:47832 ping"
