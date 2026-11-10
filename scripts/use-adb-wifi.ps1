param([string]$Ip = "")

$adb = "C:\Users\sivan\AppData\Local\Android\Sdk\platform-tools\adb.exe"

if (-not $Ip) {
    Write-Host "buscando ADB WiFi en 192.168.1.0/24:5555..."
    $clients = @()
    foreach ($i in 1..254) {
        $c = New-Object System.Net.Sockets.TcpClient
        try {
            $ar = $c.BeginConnect("192.168.1.$i", 5555, $null, $null)
            $clients += [pscustomobject]@{ ip = "192.168.1.$i"; client = $c; ar = $ar }
        } catch {
            $c.Close()
        }
    }
    Start-Sleep -Milliseconds 1200
    foreach ($x in $clients) {
        try {
            if ($x.ar.AsyncWaitHandle.WaitOne(0) -and $x.client.Connected) {
                $Ip = $x.ip
                break
            }
        } catch { }
        finally { $x.client.Close() }
    }
    $clients | ForEach-Object { try { $_.client.Close() } catch { } }
}

if (-not $Ip) {
    Write-Host "no se encontró ningún ADB WiFi"
    exit 1
}

& $adb connect "${Ip}:5555"
& $adb devices
