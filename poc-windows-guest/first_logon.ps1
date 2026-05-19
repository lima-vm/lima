#Requires -RunAsAdministrator
param(
    [Parameter(Mandatory)]
    [string]$SSHKey
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$logPath = 'C:\lima-setup.log'
Start-Transcript -Path $logPath -Append

function Write-Step {
    param([string]$Name)
    Write-Output "==> $Name"
}

function Invoke-Step {
    param([string]$Name, [scriptblock]$Action)
    Write-Step $Name
    try {
        & $Action
        Write-Output "    OK"
    } catch {
        Write-Output "    FAILED: $_"
        Stop-Transcript
        exit 1
    }
}

Invoke-Step 'Install OpenSSH Server' {
    $cap = Get-WindowsCapability -Online -Name 'OpenSSH.Server~~~~0.0.1.0'
    if ($cap.State -ne 'Installed') {
        Add-WindowsCapability -Online -Name 'OpenSSH.Server~~~~0.0.1.0'
    }
}

Invoke-Step 'Enable and start sshd' {
    Set-Service -Name sshd -StartupType Automatic
    if ((Get-Service sshd).Status -ne 'Running') {
        Start-Service sshd
    }
}

Invoke-Step 'Configure SSH firewall rule' {
    Remove-NetFirewallRule -Name 'OpenSSH-Server-In-TCP' -ErrorAction SilentlyContinue
    New-NetFirewallRule `
        -Name        'OpenSSH-Server-In-TCP' `
        -DisplayName 'OpenSSH Server (sshd)' `
        -Enabled     True `
        -Direction   Inbound `
        -Protocol    TCP `
        -Action      Allow `
        -LocalPort   22
}

Invoke-Step 'Write SSH authorised key' {
    $sshDir  = 'C:\ProgramData\ssh'
    $keyFile = Join-Path $sshDir 'administrators_authorized_keys'
    if (-not (Test-Path $sshDir)) {
        New-Item -ItemType Directory -Path $sshDir -Force | Out-Null
    }
    Set-Content -Path $keyFile -Value $SSHKey -Encoding UTF8 -Force
    icacls $keyFile /inheritance:r                | Out-Null
    icacls $keyFile /grant 'SYSTEM:(F)'           | Out-Null
    icacls $keyFile /grant 'Administrators:(F)'   | Out-Null
}

Invoke-Step 'Set default SSH shell to PowerShell' {
    $regPath = 'HKLM:\SOFTWARE\OpenSSH'
    if (-not (Test-Path $regPath)) {
        New-Item -Path $regPath -Force | Out-Null
    }
    New-ItemProperty -Path $regPath -Name DefaultShell `
        -Value 'C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe' `
        -PropertyType String -Force | Out-Null
}

Invoke-Step 'Install WinFSP (required for VirtIO-FS)' {
    [Net.ServicePointManager]::SecurityProtocol =
        [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    $chocoExe = 'C:\ProgramData\chocolatey\bin\choco.exe'
    if (-not (Test-Path $chocoExe)) {
        Set-ExecutionPolicy Bypass -Scope Process -Force
        $installScript = (New-Object Net.WebClient).DownloadString(
            'https://community.chocolatey.org/install.ps1'
        )
        Invoke-Expression $installScript
    }
    & $chocoExe install winfsp -y --pre --no-progress
}

Invoke-Step 'Register and start VirtIO-FS service' {
    $virtiofsBin = 'E:\viofs\2k25\amd64\virtiofs.exe'
    if (-not (Get-Service -Name VirtioFsSvc -ErrorAction SilentlyContinue)) {
        New-Service -Name VirtioFsSvc `
            -BinaryPathName $virtiofsBin `
            -DisplayName    'VirtIO Filesystem Service' `
            -StartupType    Automatic
    }
    if ((Get-Service VirtioFsSvc).Status -ne 'Running') {
        Start-Service VirtioFsSvc
    }
}

Write-Output '==> All steps completed successfully'
Stop-Transcript
