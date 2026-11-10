$plink = "C:\Program Files\PuTTY\plink.exe"
$pscp = "C:\Program Files\PuTTY\pscp.exe"
$key = "SHA256:lVizlC0DLx4YMwtGyU+/xBviuyMoKCSg+roUUZJTTdA"
$hosts = @("192.168.1.37", "192.168.1.52")

$cmd = @'
chmod +x ~/multirk/multirkd.new && mv -f ~/multirk/multirkd.new ~/multirk/multirkd && sed -i 's/"name": *"[^"]*"/"name": "AISLPC R36TMax"/' ~/.config/multirk/config.json && (sudo -n systemctl restart multirkd || (echo ark | sudo -S -p '' systemctl restart multirkd)) && sleep 2 && (systemctl is-active multirkd || pgrep -f multirkd) && grep -m1 name ~/.config/multirk/config.json
'@

foreach ($h in $hosts) {
    Write-Host "== $h =="
    & $pscp -batch -pw ark -hostkey $key "daemon\dist\multirkd-linux-arm64" "ark@${h}:/home/ark/multirk/multirkd.new"
    if ($LASTEXITCODE -ne 0) { Write-Host "pscp failed"; continue }
    & $plink -ssh -batch -pw ark -hostkey $key "ark@$h" $cmd
}
Write-Host "done"
