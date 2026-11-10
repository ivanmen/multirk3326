$src = "D:\source\multirk3326\prototype\assets\icons"
$dst = "D:\source\multirk3326\android\app\src\main\res\drawable-nodpi"
$names = @(
  "cozy_logo",
  "cozy_mode_one",
  "cozy_mode_two",
  "cozy_nav_devices",
  "cozy_nav_catalog",
  "cozy_nav_controls",
  "cozy_nav_settings",
  "cozy_sync"
)
foreach ($n in $names) {
  Copy-Item "$src\$n.png" "$dst\$n.png" -Force
  Write-Host "$n ok"
}
