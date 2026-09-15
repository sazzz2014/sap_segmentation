$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$project = 'sap-seg-e2e-' + [guid]::NewGuid().ToString('N').Substring(0, 10)
try {
    & docker compose -p $project up --build -d postgres mock-erp
    if ($LASTEXITCODE -ne 0) { throw 'Infrastructure startup failed' }
    & docker compose -p $project run --build --rm e2e-verify
    if ($LASTEXITCODE -ne 0) { throw 'Local E2E failed' }
    & docker compose -p $project --profile test run --rm integration
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL integration tests failed' }
} finally {
    & docker compose -p $project down --volumes --remove-orphans
    if ($LASTEXITCODE -ne 0) { Write-Warning 'Could not clean up the test Compose project' }
}
