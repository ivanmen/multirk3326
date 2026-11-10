$plink = "C:\Program Files\PuTTY\plink.exe"
$pscp = "C:\Program Files\PuTTY\pscp.exe"
$key = "SHA256:lVizlC0DLx4YMwtGyU+/xBviuyMoKCSg+roUUZJTTdA"
$cmd = "chmod +x ~/multirk/multirkd.new && mv -f ~/multirk/multirkd.new ~/multirk/multirkd && (sudo -n systemctl restart multirkd || (echo ark | sudo -S -p '' systemctl restart multirkd)) && sleep 2 && systemctl is-active multirkd"
foreach ($h in "192.168.1.37", "192.168.1.52") {
    if (-not (Test-Connection -ComputerName $h -Count 1 -Quiet)) { Write-Host "$h : sin respuesta"; continue }
    $p = Start-Process -FilePath $pscp -ArgumentList @("-batch","-q","-pw","ark","-hostkey",$key,"daemon\dist\multirkd-linux-arm64","ark@${h}:/home/ark/multirk/multirkd.new") -NoNewWindow -PassThru
    if (-not $p.WaitForExit(90000)) { $p.Kill(); Write-Host "$h : subida expiró"; continue }
    $r = & $plink -ssh -batch -pw ark -hostkey $key "ark@$h" $cmd
    Write-Host "$h : $r"
}
