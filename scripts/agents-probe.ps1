$plink = "C:\Program Files\PuTTY\plink.exe"
$pscp = "C:\Program Files\PuTTY\pscp.exe"
$hosts = @("192.168.1.37", "192.168.1.52")

foreach ($h in $hosts) {
    Write-Host "== $h =="
    $arch = ("y" | & $plink -ssh -batch -pw ark "ark@$h" "uname -m") 2>&1
    Write-Host "arch: $arch"
}
