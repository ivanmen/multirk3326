$adb = "C:\Users\sivan\AppData\Local\Android\Sdk\platform-tools\adb.exe"
& $adb tcpip 5555
Start-Sleep -Seconds 2
& $adb connect 192.168.1.5:5555
Start-Sleep -Seconds 2
& $adb devices -l
