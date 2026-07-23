param(
    [string]$Root = "F:\"
)

$ErrorActionPreference = "Stop"

$provisionDir = "C:\ProgramData\Lima\provision"
New-Item -ItemType Directory -Path $provisionDir -Force | Out-Null

# Copy startup runner and System scripts
$startupSystemRunnerSrc = Join-Path $Root "startup_runner.ps1"
$startupSystemRunnerDst = Join-Path $provisionDir "startup_runner.ps1"
Copy-Item -LiteralPath $startupSystemRunnerSrc -Destination $startupSystemRunnerDst
$startupSystemScriptsSrc = Join-Path $Root "provision.system"
$startupSystemScriptsDst = Join-Path $provisionDir "provision.system"
New-Item -ItemType Directory -Path $startupSystemScriptsDst -Force | Out-Null
foreach ($item in Get-ChildItem -LiteralPath $startupsystemScriptsSrc) {
    # `.ps1` extension is necessary for script otherwise Powershell returns an error.
    Copy-Item -LiteralPath $item.FullName -Destination (Join-Path $startupSystemScriptsDst ("{0}.ps1" -f $item.BaseName))
}

# Copy startup User scripts
$startupUserScriptsSrc = Join-Path $Root "provision.user"
$startupUserScriptsDst = Join-Path $provisionDir "provision.user"
New-Item -ItemType Directory -Path $startupUserScriptsDst -Force | Out-Null
foreach ($item in Get-ChildItem -LiteralPath $startupUserScriptsSrc) {
    # `.ps1` extension is necessary for script otherwise Powershell returns an error.
    Copy-Item -LiteralPath $item.FullName -Destination (Join-Path $startupUserScriptsDst ("{0}.ps1" -f $item.BaseName))
}

# Copy dependency scripts
$startupDependencyScriptsSrc = Join-Path $Root "provision.dependency"
$startupDependencyScriptsDst = Join-Path $provisionDir "provision.dependency"
New-Item -ItemType Directory -Path $startupDependencyScriptsDst -Force | Out-Null
foreach ($item in Get-ChildItem -LiteralPath $startupDependencyScriptsSrc) {
    # `.ps1` extension is necessary for script otherwise Powershell returns an error.
    Copy-Item -LiteralPath $item.FullName -Destination (Join-Path $startupDependencyScriptsDst ("{0}.ps1" -f $item.BaseName))
}

# Copy data files
$startupDataFilesSrc = Join-Path $Root "provision.data"
$startupDataFilesDst = Join-Path $provisionDir "provision.data"
New-Item -ItemType Directory -Path $startupDataFilesDst -Force | Out-Null
foreach ($item in Get-ChildItem -LiteralPath $startupDataFilesSrc) {
    # Data file doesn't need `.ps1` extension.
    Copy-Item -LiteralPath $item.FullName -Destination $startupDataFilesDst
}

# Set startup System task
$startupTrigger = New-ScheduledTaskTrigger -AtStartup
$startupActionSystem = New-ScheduledTaskAction -Execute "powershell.exe" -Argument ("-NoProfile -NonInteractive -ExecutionPolicy Bypass -File $startupSystemRunnerDst -Mode system")
$startupPrincipalSystem = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
Register-ScheduledTask -TaskName "LimaProvisionStartupSystem" -TaskPath "\Lima\" -Action $startupActionSystem -Trigger $startupTrigger -Principal $startupPrincipalSystem -Force | Out-Null

# Set startup User task
$startupActionUser = New-ScheduledTaskAction -Execute "powershell.exe" -Argument ("-NoProfile -NonInteractive -ExecutionPolicy Bypass -File $startupSystemRunnerDst -Mode user")
$startupPrincipalUser = New-ScheduledTaskPrincipal -UserId "{{.User}}" -LogonType ServiceAccount -RunLevel Highest
Register-ScheduledTask -TaskName "LimaProvisionStartupUser" -TaskPath "\Lima\" -Action $startupActionUser -Trigger $startupTrigger -Principal $startupPrincipalUser -Force | Out-Null
