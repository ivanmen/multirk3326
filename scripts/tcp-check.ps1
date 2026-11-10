$hosts = @("192.168.1.37", "192.168.1.52")
foreach ($h in $hosts) {
    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $iar = $client.BeginConnect($h, 47820, $null, $null)
        $ok = $iar.AsyncWaitHandle.WaitOne(3000)
        if ($ok -and $client.Connected) {
            Write-Host "$h : OPEN"
            $client.EndConnect($iar)
        } else {
            Write-Host "$h : TIMEOUT"
        }
    } catch {
        Write-Host "$h : ERROR $($_.Exception.Message)"
    } finally {
        $client.Close()
    }
}
