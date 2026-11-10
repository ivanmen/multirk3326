$adb = "C:\Users\sivan\AppData\Local\Android\Sdk\platform-tools\adb.exe"
New-Item -ItemType Directory -Force -Path "prototype\assets\screens" | Out-Null
& $adb install -r "android\app\build\outputs\apk\debug\app-debug.apk"
& $adb shell am force-stop org.multirk.app
& $adb shell am start -n "org.multirk.app/.MainActivity"
Start-Sleep -Seconds 4
& $adb shell screencap -p /sdcard/multirk.png
& $adb pull /sdcard/multirk.png "prototype\assets\screens\app-devices.png"
Write-Host "ok"
