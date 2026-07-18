$logfile = "C:\Users\{{.User}}\lima-setup.log"
$globalStart = Get-Date

# Record logs
Start-Transcript -Path $logfile -Append

Write-Output "Lima Setup Started at $globalStart"

$sectionStart = Get-Date
# We need to change password because the current password is specified in autounattend.xml, so all users/processes can see it.
# Generate a random 16 character password.
# Avoid special characters to minimize potential keyboard layout issue.
$chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
$newPassword = -join ((1..16) | ForEach-Object { $chars[(Get-Random -Maximum $chars.Length)] })

# Store the password under the user directory so that user can know/change it.
[System.IO.File]::WriteAllText("C:\Users\{{.User}}\password.txt", $newPassword, (New-Object System.Text.UTF8Encoding $false))

# Change the password
$username = $env:USERNAME
$newSecurePassword = ConvertTo-SecureString $newPassword -AsPlainText -Force
Set-LocalUser -Name $username -Password $newSecurePassword

$elapsed = (Get-Date) - $sectionStart
Write-Output "Password change completed in $($elapsed.TotalSeconds) seconds"


$sectionStart = Get-Date
# Install OpenSSH via MSI installer
# Avoid using the OpenSSH.Server included in the OS because installing OpenSSH takes much longer than Win32-OpenSSH.
$installer = "C:\Users\{{.User}}\openssh.msi"
{{ $openSSHInstaller := "https://github.com/PowerShell/Win32-OpenSSH/releases/download/10.0.0.0p2-Preview/OpenSSH-Win64-v10.0.0.0.msi" -}}
{{ $openSSHInstallerSHA256 := "ddec9c53864280759cf9f74791cefd387100e3946aa849a1c138a4ed1b96b7d9" -}}
{{ if eq .Arch "aarch64" -}}
{{ $openSSHInstaller = "https://github.com/PowerShell/Win32-OpenSSH/releases/download/10.0.0.0p2-Preview/OpenSSH-ARM64-v10.0.0.0.msi" -}}
{{ $openSSHInstallerSHA256 = "7a17d0e22d004fb47ca4bfd8fef926fa305de4ebf70a6f3c7a29c39aabef0023" -}}
{{ end -}}
Invoke-WebRequest -Uri "{{ $openSSHInstaller }}" -OutFile $installer

# Expected hash comes from https://github.com/powershell/win32-openssh/releases
$expectedHash = "{{ $openSSHInstallerSHA256 }}".ToLower()
$actualHash = (Get-FileHash -Path $installer -Algorithm SHA256).Hash.ToLower()

# Verify the integrity of the downloaded installer before executing it.
if ($actualHash -eq $expectedHash) {
    Write-Output "OpenSSH SHA256 verification succeeded"
}else{
    Write-Output "OpenSSH SHA256 verification failed. Expected: ${expectedHash}, Actual: ${actualHash}"
    Stop-Transcript
    exit 1
}

msiexec /i $installer ADDLOCAL=Server
[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path",[System.EnvironmentVariableTarget]::Machine) + ';' + ${Env:ProgramFiles} + '\OpenSSH', [System.EnvironmentVariableTarget]::Machine)
Get-Service -Name ssh*

$elapsed = (Get-Date) - $sectionStart
Write-Output "OpenSSH server installation completed in $($elapsed.TotalSeconds) seconds"

$sectionStart = Get-Date
# Modify firewall rule
# Note that Windows server may have a firewall rule for SSH by default, but it doesn't work on my env.
# So I remove and recreate the rule.
Remove-NetFirewallRule -Name 'OpenSSH-Server-In-TCP' -ErrorAction Ignore
New-NetFirewallRule -Name 'OpenSSH-Server-In-TCP' -DisplayName 'OpenSSH Server (sshd)' -Enabled True -Direction Inbound -Protocol TCP -Action Allow -LocalPort 22

# Set a public key. Since a user `lima` is in Administrators group,
# The public key should be located under C:\ProgramData\ssh instead of under C:\Users\lima\.ssh.
$pubkey = Get-Content -Path F:\ssh_authorized_keys
$pubkeyLocation = 'C:\ProgramData\ssh\administrators_authorized_keys'
Add-Content -Force -Path $pubkeyLocation -Value $pubkey
icacls $pubkeyLocation /inheritance:r
icacls $pubkeyLocation /grant "SYSTEM:F"
icacls $pubkeyLocation /grant "Administrators:F"

$elapsed = (Get-Date) - $sectionStart
Write-Output "SSH setting completed in $($elapsed.TotalSeconds) seconds"

# Finally, it creates a marker file. Host agent will check the file.
$bootDoneDir = 'C:\ProgramData\Lima'
New-Item -ItemType Directory -Path $bootDoneDir -Force | Out-Null
$bootDoneFile = Join-Path $bootDoneDir 'lima-boot-done.txt'
'done' | Out-File -FilePath $bootDoneFile -Encoding ascii -Force -NoNewline


$elapsed = (Get-Date) - $globalStart
Write-Output "All works completed in $($elapsed.TotalSeconds) seconds"

# Finish recording logs
Stop-Transcript
