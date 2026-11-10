$plink = "C:\Program Files\PuTTY\plink.exe"
$key = "SHA256:lVizlC0DLx4YMwtGyU+/xBviuyMoKCSg+roUUZJTTdA"
$cmd = "systemctl is-active multirkd; ip -4 addr show wlan0 | grep inet; ip link show wlan0 | head -1; journalctl -u multirkd -n 15 --no-pager 2>/dev/null | cut -c1-160; iw dev wlan0 get power_save 2>/dev/null"
foreach ($h in @("192.168.1.52", "192.168.1.37")) {
    Write-Host "== $h =="
    & $plink -ssh -batch -pw ark -hostkey $key "ark@$h" $cmd
    Write-Host "exit=$LASTEXITCODE"
}
