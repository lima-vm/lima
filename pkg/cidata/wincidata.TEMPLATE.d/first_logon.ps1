$logfile = "C:\Users\{{.User}}\lima-setup.log"

# Record logs
Start-Transcript -Path $logfile -Append

# We need to change password because the current password is specified in autounattend.xml, so all users/processes can see it.
## Generate a random 16 character password.
## Avoid special characters to minimize potential keyboard layout issue.
$chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
$newPassword = -join ((1..16) | ForEach-Object { $chars[(Get-Random -Maximum $chars.Length)] })

## Store the password under the user directory so that user can know/change it.
$newPassword | Out-File -FilePath "C:\Users\{{.User}}\password.txt" -Encoding utf8 -NoNewline

## Change the password
$username = $env:USERNAME
$newSecurePassword = ConvertTo-SecureString $newPassword -AsPlainText -Force
Set-LocalUser -Name $username -Password $newSecurePassword

# Install OpenSSH server, then enable it
Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
Start-Service sshd
Set-Service -Name sshd -StartupType Automatic

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

# Install chocolatey for installing WinFSP.
# WinFSP can be installed through winget as well, but currently winget is unstable in Windows Server Core
# See: https://github.com/microsoft/winget-cli/discussions/5230
Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))

# Install WinFSP for VirtioFS
C:\ProgramData\chocolatey\choco.exe install winfsp -y --pre

# Create VirtioFs service from virtio-win, and enable it
# By default, the host directory is mounted on Z:
New-Service -Name VirtioFsSvc -BinaryPathName 'E:\viofs\2k25\amd64\virtiofs.exe' -DisplayName VirtioFsSvc -StartupType Automatic
Start-Service -Name VirtioFsSvc

# Finish recording logs
Stop-Transcript
