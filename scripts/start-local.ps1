$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
Set-Location $projectRoot
if (!(Test-Path .env)) { throw 'Copy .env.example to .env and configure its secrets.' }
Get-Content .env | ForEach-Object {
    if ($_ -match '^([A-Z_]+)=(.*)$') { [Environment]::SetEnvironmentVariable($Matches[1], $Matches[2], 'Process') }
}
if (!(Test-Path .venv/Scripts/python.exe)) { throw 'Install the Python environment described in README.md first.' }
foreach ($port in @(8080, 8091)) {
    if (Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue) {
        throw "Port $port is already in use. Stop the existing local services first."
    }
}
New-Item -ItemType Directory -Force var, bin | Out-Null
go run ./cmd/migrate
if ($LASTEXITCODE) { throw 'Database migration failed.' }
go build -o bin/erp.exe ./cmd/erp
if ($LASTEXITCODE) { throw 'Go build failed.' }
$started = @()
try {
    $ai = Start-Process -FilePath "$projectRoot/.venv/Scripts/python.exe" -ArgumentList '-m uvicorn app.main:app --app-dir ai-service --host 127.0.0.1 --port 8091' -WorkingDirectory $projectRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput var/ai.log -RedirectStandardError var/ai-error.log
    $started += $ai
    $erp = Start-Process -FilePath "$projectRoot/bin/erp.exe" -WorkingDirectory $projectRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput var/erp.log -RedirectStandardError var/erp-error.log
    $started += $erp
    $started | ForEach-Object { [pscustomobject]@{ Id = $_.Id; StartTicks = $_.StartTime.ToUniversalTime().Ticks } } | ConvertTo-Json | Set-Content var/processes.json
    $ready = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        try {
            Invoke-RestMethod http://127.0.0.1:8080/health/ready -TimeoutSec 2 | Out-Null
            Invoke-RestMethod http://127.0.0.1:8091/health/ready -TimeoutSec 2 | Out-Null
            $ready = $true
            break
        } catch { Start-Sleep -Seconds 1 }
    }
    if (!$ready) { throw 'Services did not become ready. Inspect var/*-error.log.' }
    Write-Output 'PanlyackERP is ready: http://127.0.0.1:8080/app/dashboard'
} catch {
    if (Test-Path var/processes.json) { & "$PSScriptRoot/stop-local.ps1" }
    foreach ($service in $started) { if (!$service.HasExited) { Stop-Process -Id $service.Id -ErrorAction SilentlyContinue } }
    throw
}

