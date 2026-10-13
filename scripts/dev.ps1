param([switch]$Check)
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
if (!(Test-Path .env)) { throw 'Create .env from .env.example and fill its secrets first.' }
Get-Content .env | ForEach-Object {
    if ($_ -match '^([A-Z_]+)=(.*)$') { [Environment]::SetEnvironmentVariable($Matches[1],$Matches[2],'Process') }
}
if ($Check) {
    go test ./...
    if ($LASTEXITCODE) { exit $LASTEXITCODE }
    go vet ./...
    exit $LASTEXITCODE
}
go run ./cmd/erp
