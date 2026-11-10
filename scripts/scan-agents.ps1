$jobs = 1..254 | ForEach-Object {
    $ip = "192.168.1.$_"
    $c = New-Object Net.Sockets.TcpClient
    [pscustomobject]@{ ip = $ip; c = $c; t = $c.ConnectAsync($ip, 47820) }
}
Start-Sleep -Seconds 2
$jobs | Where-Object { $_.c.Connected } | ForEach-Object { "agent: " + $_.ip }
