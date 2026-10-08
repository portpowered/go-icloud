$ErrorActionPreference = 'Stop'
$referenceSources = Get-Content -LiteralPath "$PSScriptRoot/tools/reference/source.json" -Raw | ConvertFrom-Json
foreach ($referenceName in @('historical', 'live')) {
    $referenceSource = $referenceSources.$referenceName
    $checkoutName = if ($referenceName -eq 'live') { 'pyicloud-live' } else { 'pyicloud' }
    $checkoutPath = Join-Path $PSScriptRoot ".reference/$checkoutName"
    if (-not (Test-Path -LiteralPath $checkoutPath)) {
        git clone $referenceSource.url $checkoutPath
        if ($LASTEXITCODE -ne 0) { throw 'Reference clone failed' }
    }
    $referenceDirty = git -C $checkoutPath status --porcelain
    if ($LASTEXITCODE -ne 0 -or $referenceDirty) { throw 'Preserve local reference edits before setup' }
    git -C $checkoutPath checkout --detach $referenceSource.commit
    if ($LASTEXITCODE -ne 0) { throw 'Pinned reference checkout failed' }
}
if (-not (Test-Path -LiteralPath "$PSScriptRoot/.venv/Scripts/python.exe")) {
    python -m venv "$PSScriptRoot/.venv"
    if ($LASTEXITCODE -ne 0) { throw 'Virtual environment creation failed' }
}
& "$PSScriptRoot/.venv/Scripts/python.exe" -m pip install -r "$PSScriptRoot/tools/reference/requirements.lock"
if ($LASTEXITCODE -ne 0) { throw 'Dependency installation failed' }
& "$PSScriptRoot/.venv/Scripts/python.exe" -m pip install --no-deps -e "$PSScriptRoot/.reference/pyicloud-live"
if ($LASTEXITCODE -ne 0) { throw 'Reference installation failed' }
Write-Host 'Ready. Run .\icloud.ps1 login in your terminal.'
