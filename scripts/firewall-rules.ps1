$rules = @(
    @{ Name = "MULTIRK TCP 47820-47832"; Proto = "TCP"; Ports = "47820-47832" },
    @{ Name = "MULTIRK UDP 47820 discovery"; Proto = "UDP"; Ports = "47820" }
)
foreach ($r in $rules) {
    try {
        $existing = Get-NetFirewallRule -DisplayName $r.Name -ErrorAction SilentlyContinue
        if (-not $existing) {
            New-NetFirewallRule -DisplayName $r.Name -Direction Inbound -Action Allow `
                -Protocol $r.Proto -LocalPort $r.Ports -Profile Private,Public -ErrorAction Stop | Out-Null
            Write-Host "regla creada: $($r.Name)"
        } else {
            Write-Host "regla existente: $($r.Name)"
        }
    } catch {
        Write-Host "FALLO '$($r.Name)': ejecuta PowerShell como administrador"
    }
}
