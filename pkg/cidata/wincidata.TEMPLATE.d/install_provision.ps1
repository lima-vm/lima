param(
    [string]$Root = "F:\"
)

$ErrorActionPreference = "Stop"

function Copy-Provision {
    param(
        [string]$Src,
        [string]$Dst,
        [bool]$Extension
    )

    if (-not (Test-Path -LiteralPath $Src)) {
        return
    }

    New-Item -ItemType Directory -Path $Dst -Force | Out-Null
    foreach ($item in Get-ChildItem -LiteralPath $Src) {
        $dst = $Dst
        if ($Extension) {
            # `.ps1` extension is necessary for script otherwise Powershell returns an error.
            $dst = Join-Path $Dst ("{0}.ps1" -f $item.BaseName)
        }
        Copy-Item -LiteralPath $item.FullName -Destination $dst
    }
}

$provisionDir = "C:\ProgramData\Lima\provision"
New-Item -ItemType Directory -Path $provisionDir -Force | Out-Null

# Copy startup runner and System scripts
$startupSystemRunnerSrc = Join-Path $Root "startup_runner.ps1"
$startupSystemRunnerDst = Join-Path $provisionDir "startup_runner.ps1"
Copy-Item -LiteralPath $startupSystemRunnerSrc -Destination $startupSystemRunnerDst

$startupSystemScriptsSrc = Join-Path $Root "provision.system"
$startupSystemScriptsDst = Join-Path $provisionDir "provision.system"
Copy-Provision -Src $startupSystemScriptsSrc -Dst $startupSystemScriptsDst -Extension

# Copy startup User scripts
$startupUserScriptsSrc = Join-Path $Root "provision.user"
$startupUserScriptsDst = Join-Path $provisionDir "provision.user"
Copy-Provision -Src $startupUserScriptsSrc -Dst $startupUserScriptsDst -Extension

# Copy dependency scripts
$startupDependencyScriptsSrc = Join-Path $Root "provision.dependency"
$startupDependencyScriptsDst = Join-Path $provisionDir "provision.dependency"
Copy-Provision -Src $startupDependencyScriptsSrc -Dst $startupDependencyScriptsDst -Extension


# Copy data files
$startupDataFilesSrc = Join-Path $Root "provision.data"
$startupDataFilesDst = Join-Path $provisionDir "provision.data"
Copy-Provision -Src $startupDataFilesSrc -Dst $startupDataFilesDst


# Set startup System task
$startupTrigger = New-ScheduledTaskTrigger -AtStartup
$startupActionSystem = New-ScheduledTaskAction -Execute "powershell.exe" -Argument ("-NoProfile -NonInteractive -ExecutionPolicy Bypass -File $startupSystemRunnerDst -Mode system")
$startupPrincipalSystem = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
Register-ScheduledTask -TaskName "LimaProvisionStartupSystem" -TaskPath "\Lima\" -Action $startupActionSystem -Trigger $startupTrigger -Principal $startupPrincipalSystem -Force | Out-Null

# Set startup User task
$startupActionUser = New-ScheduledTaskAction -Execute "powershell.exe" -Argument ("-NoProfile -NonInteractive -ExecutionPolicy Bypass -File $startupSystemRunnerDst -Mode user")
$startupPrincipalUser = New-ScheduledTaskPrincipal -UserId "{{.User}}" -LogonType S4U -RunLevel Limited
Register-ScheduledTask -TaskName "LimaProvisionStartupUser" -TaskPath "\Lima\" -Action $startupActionUser -Trigger $startupTrigger -Principal $startupPrincipalUser -Force | Out-Null
