param([switch]$Check)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
Set-Location $projectRoot
if (!(Test-Path .env)) { throw 'Copy .env.example to .env and configure its secrets.' }
Get-Content .env | ForEach-Object {
    if ($_ -match '^([A-Z_]+)=(.*)$') { [Environment]::SetEnvironmentVariable($Matches[1], $Matches[2], 'Process') }
}
$aiPython = if ($env:AI_PYTHON) { $env:AI_PYTHON } else { Join-Path $projectRoot '.venv/Scripts/python.exe' }
if (!(Test-Path -LiteralPath $aiPython)) { throw 'Install the configured AI_PYTHON environment described in README.md first.' }
$aiPython = (Resolve-Path -LiteralPath $aiPython).Path
$erpAddress = if ($env:HTTP_ADDR) { $env:HTTP_ADDR } else { '127.0.0.1:8080' }
$aiAddress = if ($env:AI_BASE_URL) { $env:AI_BASE_URL } else { 'http://127.0.0.1:8091' }
$erpUri = [uri]"http://$erpAddress"
$aiUri = [uri]$aiAddress
foreach ($endpoint in @($erpUri, $aiUri)) {
    if ($endpoint.Scheme -ne 'http' -or $endpoint.DnsSafeHost -notin @('127.0.0.1', 'localhost', '::1') -or $endpoint.AbsolutePath -ne '/' -or $endpoint.Query -or $endpoint.Fragment -or $endpoint.UserInfo) {
        throw 'HTTP_ADDR and AI_BASE_URL must name plain loopback HTTP endpoints for the local launcher.'
    }
}
if ($erpUri.Port -eq $aiUri.Port) { throw 'ERP and AI must use different local ports.' }
$erpUrl = $erpUri.GetLeftPart([System.UriPartial]::Authority)
$aiUrl = $aiUri.GetLeftPart([System.UriPartial]::Authority)
foreach ($endpoint in @($erpUri, $aiUri)) {
    $port = $endpoint.Port
    if (Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue) {
        throw "Port $port is already in use. Stop the existing local services first."
    }
    $address = if ($endpoint.DnsSafeHost -eq '::1') { [System.Net.IPAddress]::IPv6Loopback } else { [System.Net.IPAddress]::Loopback }
    $listener = [System.Net.Sockets.TcpListener]::new($address, $port)
    try { $listener.Start() } catch { throw "Port $port cannot be bound (it may be reserved by Windows). Choose another port in .env." } finally { $listener.Stop() }
}
if ($Check) { Write-Output "Local preflight passed: ERP $erpUrl; AI $aiUrl"; return }
New-Item -ItemType Directory -Force var, bin | Out-Null
go run ./cmd/migrate
if ($LASTEXITCODE) { throw 'Database migration failed.' }
go build -o bin/erp.exe ./cmd/erp
if ($LASTEXITCODE) { throw 'Go build failed.' }
$started = @()
try {
    $aiArguments = "-m uvicorn app.main:app --app-dir ai-service --host $($aiUri.DnsSafeHost) --port $($aiUri.Port)"
    $ai = Start-Process -FilePath $aiPython -ArgumentList $aiArguments -WorkingDirectory $projectRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput var/ai.log -RedirectStandardError var/ai-error.log
    $started += $ai
    $erp = Start-Process -FilePath "$projectRoot/bin/erp.exe" -WorkingDirectory $projectRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput var/erp.log -RedirectStandardError var/erp-error.log
    $started += $erp
    $started | ForEach-Object { [pscustomobject]@{ Id = $_.Id; StartTicks = $_.StartTime.ToUniversalTime().Ticks } } | ConvertTo-Json | Set-Content var/processes.json
    $ready = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        if ($ai.HasExited -or $erp.HasExited) { throw 'A local service exited during startup. Inspect var/ai-error.log and var/erp.log.' }
        try {
            Invoke-RestMethod "$erpUrl/health/ready" -TimeoutSec 2 | Out-Null
            Invoke-RestMethod "$aiUrl/health/ready" -TimeoutSec 2 | Out-Null
            $ready = $true
            break
        } catch { Start-Sleep -Seconds 1 }
    }
    if (!$ready) { throw 'Services did not become ready. Inspect var/*-error.log.' }
    Write-Output "PanlyackERP is ready: $erpUrl/app/dashboard (AI: $aiUrl)"
} catch {
    if (Test-Path var/processes.json) { & "$PSScriptRoot/stop-local.ps1" }
    foreach ($service in $started) { if (!$service.HasExited) { Stop-Process -Id $service.Id -ErrorAction SilentlyContinue } }
    throw
}

