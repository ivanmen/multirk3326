param(
    [string]$HostIp = "",
    [int]$Port = 47832,
    [switch]$NoLaunch
)
$adb = "C:\Users\sivan\AppData\Local\Android\Sdk\platform-tools\adb.exe"
$root = Split-Path -Parent $PSScriptRoot
$apk = Join-Path $root "android\app\build\outputs\apk\debug\app-debug.apk"

$list = & $adb devices
$physical = @($list | Where-Object { $_ -match "^\S+\s+device" -and $_ -notmatch "emulator" })
if ($physical.Count -eq 0) {
    Write-Host "No hay un telefono fisico autorizado."
    Write-Host ""
    Write-Host "1. En el telefono: Ajustes > Acerca de > toca 7 veces 'Numero de compilacion'."
    Write-Host "2. Ajustes > Sistema > Opciones de desarrollador > Depuracion USB ON."
    Write-Host "3. Conecta el cable USB y acepta 'Permitir depuracion USB'."
    Write-Host "4. Vuelve a ejecutar este script."
    Write-Host ""
    Write-Host "Tambien puedes usar ADB inalambrico (Android 11+):"
    Write-Host "   adb pair IP:PUERTO   (codigo de 6 digitos)"
    Write-Host "   adb connect IP:5555"
    exit 1
}

Write-Host "Instalando $apk ..."
& $adb install -r $apk
if ($LASTEXITCODE -ne 0) { exit 1 }

if (-not $NoLaunch) {
    if ($HostIp) {
        & $adb shell am force-stop org.multirk.app
        & $adb shell am start -n "org.multirk.app/.MainActivity" --es host $HostIp --ei port $Port
    } else {
        & $adb shell am start -n "org.multirk.app/.MainActivity"
    }
}

Write-Host ""
Write-Host "IPs de este PC (usa la de la misma red que el telefono):"
Get-NetIPAddress -AddressFamily IPv4 |
    Where-Object { $_.IPAddress -notlike "127.*" -and $_.IPAddress -notlike "169.254.*" } |
    Select-Object IPAddress, InterfaceAlias | Format-Table -AutoSize
