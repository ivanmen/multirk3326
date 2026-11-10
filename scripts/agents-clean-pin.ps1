$plink = "C:\Program Files\PuTTY\plink.exe"
$key = "SHA256:lVizlC0DLx4YMwtGyU+/xBviuyMoKCSg+roUUZJTTdA"
foreach ($h in "192.168.1.37", "192.168.1.52") {
    Write-Host "== $h =="
    & $plink -ssh -batch -pw ark -hostkey $key "ark@$h" "rm -f ~/.config/multirk/pin.txt; sed -i '/\`"pin\`"/d' ~/.config/multirk/runtime.json; ls ~/.config/multirk; grep -c pin ~/.config/multirk/runtime.json"
}
