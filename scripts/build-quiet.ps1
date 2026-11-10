$out = powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-android.ps1 assembleDebug --console=plain 2>&1 | Out-String
$lines = $out -split "`r?`n"
$errs = $lines | Where-Object { $_ -match '^e: ' } | Select-Object -First 25
$errs | ForEach-Object { Write-Host $_ }
if ($out -match 'BUILD SUCCESSFUL') { Write-Host "RESULT: BUILD SUCCESSFUL" }
elseif ($out -match 'BUILD FAILED') { Write-Host "RESULT: BUILD FAILED ($($errs.Count) errors shown)" }
