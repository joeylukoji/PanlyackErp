param([string]$Email = 'owner@panlyack.local')
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
if (!(Test-Path .env)) { throw 'Configure .env first.' }
Get-Content .env | ForEach-Object {
    if ($_ -match '^([A-Z_]+)=(.*)$') { [Environment]::SetEnvironmentVariable($Matches[1], $Matches[2], 'Process') }
}
$securePassword = Read-Host 'Choose an owner password (12 to 128 characters)' -AsSecureString
$env:OWNER_EMAIL = $Email
$env:OWNER_PASSWORD = [System.Net.NetworkCredential]::new('', $securePassword).Password
try {
    go run ./cmd/owner
    if ($LASTEXITCODE) { throw 'Owner account setup failed.' }
} finally {
    Remove-Item Env:OWNER_PASSWORD -ErrorAction SilentlyContinue
    Remove-Item Env:OWNER_EMAIL -ErrorAction SilentlyContinue
}
