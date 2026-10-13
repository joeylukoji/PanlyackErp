param([string]$Email = 'owner@panlyack.local')
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
if (!(Test-Path .env)) { throw 'Configure .env first.' }
$securePassword = Read-Host 'Choose an owner password (12 to 128 characters)' -AsSecureString
$password = [System.Net.NetworkCredential]::new('', $securePassword).Password
$env:OWNER_EMAIL = $Email
try {
    $password | docker compose --env-file .env -f deployments/compose.yaml exec -T -e OWNER_EMAIL erp owner
    if ($LASTEXITCODE) { throw 'Container owner setup failed.' }
} finally {
    Remove-Item Env:OWNER_EMAIL -ErrorAction SilentlyContinue
    $password = $null
}
