$uas = @(
  "Wget/1.20.3 (linux-gnu)",
  "Mozilla/5.0 (Windows NT 6.1; WOW64; rv:27.0) Gecko/20100101 Firefox/27.0",
  "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36",
  "curl/7.68.0"
)
foreach ($ua in $uas) {
  try {
    $css = Invoke-WebRequest -Uri "https://fonts.googleapis.com/css?family=Press+Start+2P" -Headers @{ "User-Agent" = $ua } -UseBasicParsing -TimeoutSec 20
    $m = [regex]::Match($css.Content, 'https://fonts\.gstatic\.com/[^)]+\.(ttf|woff2?|eot)')
    if ($m.Success) {
      Write-Host "$ua -> $($m.Value)"
    } else {
      Write-Host "$ua -> sin match"
    }
  } catch {
    Write-Host "$ua -> error"
  }
}
