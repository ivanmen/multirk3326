$found = @()
$clients = @()
foreach ($i in 1..254) {
    $ip = "192.168.1.$i"
    $c = New-Object System.Net.Sockets.TcpClient
    try {
        $ar = $c.BeginConnect($ip, 5555, $null, $null)
        $clients += [pscustomobject]@{ ip = $ip; client = $c; ar = $ar }
    } catch {
        $c.Close()
    }
}
Start-Sleep -Milliseconds 1200
foreach ($x in $clients) {
    try {
        if ($x.ar.AsyncWaitHandle.WaitOne(0) -and $x.client.Connected) {
            $found += $x.ip
        }
    } catch { }
    $x.client.Close()
}
if ($found.Count -eq 0) {
    Write-Host "no adb-over-wifi found on 192.168.1.0/24:5555"
} else {
    $found | ForEach-Object { Write-Host "adb tcp candidate: $_" }
}
