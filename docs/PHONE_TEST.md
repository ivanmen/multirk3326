# Probar en un teléfono real

El emulador sirve para validar la UI, pero el rendimiento y el WiFi reales solo
se prueban en el teléfono.

## 1. Preparar el teléfono

1. Ajustes → Acerca del teléfono → toca 7 veces **Número de compilación**.
2. Ajustes → Sistema → **Opciones de desarrollador** → **Depuración USB** ON.
3. Conecta el cable USB y acepta **Permitir depuración USB**.

Comprueba:

```powershell
C:\Users\sivan\AppData\Local\Android\Sdk\platform-tools\adb.exe devices
```

Debe aparecer tu teléfono como `device` (no `unauthorized`).

### Alternativa inalámbrica (Android 11+)

```powershell
adb pair 192.168.1.50:37123     # IP del teléfono y código de 6 dígitos
adb connect 192.168.1.50:5555
```

## 2. Instalar y abrir

```powershell
powershell -File scripts\install-phone.ps1
```

APK: `android\app\build\outputs\apk\debug\app-debug.apk` (12 MB, firmado con
la clave debug; sirve para pruebas, no para publicar).

## 3. Daemon de prueba en el PC

```powershell
powershell -File scripts\start-test-daemon.ps1
```

Imprime la IP del PC. Deja `multirkd` escuchando en
`0.0.0.0:47832` con el catálogo de `testdata/`.

Si el teléfono no conecta, abre el puerto en el Firewall de Windows
(PowerShell como administrador):

```powershell
powershell -File scripts\firewall-rules.ps1
```

## 4. Conectar desde la app

En la app:

1. **AÑADIR POR IP**
2. IP del PC (la de la misma WiFi, p. ej. `192.168.1.20`)
3. Puerto `47832`
4. **ABRIR CATÁLOGO**

También puedes lanzar la app ya conectada:

```powershell
adb shell am start -n org.multirk.app/.MainActivity `
  --es host 192.168.1.20 --ei port 47832
```

## 5. Con las consolas reales

En la consola:

```bash
multirkctl install -host 192.168.1.42 -key ~/.ssh/id_ed25519
```

En la app: **AÑADIR POR IP** → IP de la consola → puerto `47820`.

El descubrimiento multicast (`239.255.42.99:47820`) debería funcionar si el
router no aísla clientes; si no, usa IP manual.

## Problemas típicos

| síntoma | causa | solución |
|---|---|---|
| `adb devices` vacío | depuración USB apagada | activar y aceptar el diálogo |
| `unauthorized` | no aceptaste la huella | revocar autorizaciones USB y reconectar |
| app conecta y no carga | firewall del PC | `scripts\firewall-rules.ps1` |
| no aparece por multicast | AP isolation / VLAN | añadir por IP |
| volumen/billo no responde | permisos ALSA/sysfs | ver README (grupo video, udev) |

## Logs

```powershell
adb logcat -s AndroidRuntime:E
adb logcat | Select-String "multirk"
```
