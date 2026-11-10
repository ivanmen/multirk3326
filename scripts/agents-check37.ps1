$plink = "C:\Program Files\PuTTY\plink.exe"
$key = "SHA256:lVizlC0DLx4YMwtGyU+/xBviuyMoKCSg+roUUZJTTdA"
& $plink -ssh -batch -pw ark -hostkey $key "ark@192.168.1.37" "systemctl is-active multirkd; uptime"
Write-Host "exit=$LASTEXITCODE"
