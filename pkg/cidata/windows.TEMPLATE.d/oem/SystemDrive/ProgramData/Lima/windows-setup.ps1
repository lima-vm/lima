$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$limaDir = Join-Path $env:ProgramData 'Lima'
$sourceKeys = Join-Path $limaDir 'ssh_authorized_keys'
$targetKeys = Join-Path $env:ProgramData 'ssh\administrators_authorized_keys'
$targetKeysDir = Split-Path -Parent $targetKeys
$setupDone = Join-Path $limaDir 'setup.done'

Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0

New-ItemProperty -Path 'HKLM:\SOFTWARE\OpenSSH' -Name DefaultShell -Value "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe" -PropertyType String -Force | Out-Null

Set-Service -Name sshd -StartupType Automatic
$sshdService = Get-Service -Name sshd
if ($sshdService.Status -ne 'Running') {
    try {
        Start-Service -Name sshd
        $sshdService = Get-Service -Name sshd
        $sshdService.WaitForStatus('Running', (New-TimeSpan -Minutes 5))
    } catch {
        throw "sshd did not start: $($_.Exception.Message)"
    }
    $sshdService = Get-Service -Name sshd
    if ($sshdService.Status -ne 'Running') {
        throw "sshd did not reach Running state; current state is $($sshdService.Status)"
    }
}
if (-not (Test-Path -LiteralPath $targetKeysDir -PathType Container)) {
    throw "OpenSSH did not create $targetKeysDir"
}

Copy-Item -Force $sourceKeys $targetKeys

icacls.exe $targetKeys /inheritance:r | Out-Null
icacls.exe $targetKeys /grant '*S-1-5-18:F' '*S-1-5-32-544:F' | Out-Null

$ethernetProfile = Get-NetConnectionProfile |
    Where-Object { $_.InterfaceAlias -like 'Ethernet*' } |
    Select-Object -First 1
if ($null -ne $ethernetProfile -and $ethernetProfile.NetworkCategory -ne 'Private') {
    Set-NetConnectionProfile -InterfaceIndex $ethernetProfile.InterfaceIndex -NetworkCategory Private
}
$openSSHRule = Get-NetFirewallRule -Name 'OpenSSH-Server-In-TCP' -ErrorAction SilentlyContinue
if ($null -ne $openSSHRule) {
    Enable-NetFirewallRule -Name 'OpenSSH-Server-In-TCP'
} else {
    New-NetFirewallRule -Name 'OpenSSH-Server-In-TCP' -DisplayName 'OpenSSH Server (sshd)' -Enabled True -Direction Inbound -Protocol TCP -Action Allow -LocalPort 22 -Profile Private | Out-Null
}

Set-Content -Path $setupDone -Value (Get-Date -Format o)
