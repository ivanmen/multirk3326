$plink = "C:\Program Files\PuTTY\plink.exe"
$key = "SHA256:lVizlC0DLx4YMwtGyU+/xBviuyMoKCSg+roUUZJTTdA"
$hosts = @("192.168.1.37", "192.168.1.52")

$cmd = "sudo -n systemctl restart multirkd || (echo ark | sudo -S -p '' systemctl restart multirkd); sleep 2; systemctl is-active multirkd; ~/multirk/multirkd -version 2>/dev/null || true"

foreach ($h in $hosts) {
    Write-Host "== $h =="
    & $plink -ssh -batch -pw ark -hostkey $key "ark@$h" $cmd
}
Write-Host "done"
