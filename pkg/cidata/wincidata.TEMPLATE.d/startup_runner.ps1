param(
    [string]$Mode = "system"
)

function Get-ProvisionFiles {
    param(
        [string]$Root,
        [string]$Mode
    )
    $dir = Join-Path $Root ("provision.{0}" -f $Mode)
    if (-not (Test-Path -LiteralPath $dir)) {
        return @()
    }
    return @(Get-ChildItem -LiteralPath $dir -File | Sort-Object Name)
}

function Invoke-ProvisionScriptFile {
    param(
        [System.IO.FileInfo]$File
    )

    powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $File.FullName
    if ($LASTEXITCODE -ne 0) {
        Write-Error ("Provision script exited with code {0}: {1}" -f $LASTEXITCODE, $File.FullName)
    }
    return $LASTEXITCODE
}

$baseDir = "C:\ProgramData\Lima\provision"
$exitCode = 0
foreach ($f in (Get-ProvisionFiles -Root $baseDir -Mode $Mode)) {
    $code = Invoke-ProvisionScriptFile -File $f
    if ($code -ne 0) {
        $exitCode = $code
    }
}
exit $exitCode
