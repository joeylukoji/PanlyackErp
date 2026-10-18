$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$previousLocation = Get-Location
$testRoot = Join-Path $projectRoot ('var/launcher-preflight-' + [guid]::NewGuid().ToString('N'))
$scriptFolder = Join-Path $testRoot 'scripts'
New-Item -ItemType Directory -Path $scriptFolder -Force | Out-Null
$launcher = Join-Path $scriptFolder 'start-local.ps1'
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'start-local.ps1') -Destination $launcher
$python = (Resolve-Path (Join-Path $projectRoot '.venv/Scripts/python.exe')).Path
$configFile = Join-Path $testRoot '.env'
function Write-TestConfiguration([int]$erpPort, [int]$aiPort, [string]$aiHost = '127.0.0.1') {
    @("AI_PYTHON=$python", "HTTP_ADDR=127.0.0.1:$erpPort", "AI_BASE_URL=http://${aiHost}:$aiPort") | Set-Content -LiteralPath $configFile
}
function Get-TestPort {
    $probe = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
    try { $probe.Start(); return $probe.LocalEndpoint.Port } finally { $probe.Stop() }
}
function Expect-Rejection([string]$message) {
    $failure = $null
    try { & $launcher -Check | Out-Null } catch { $failure = $_.Exception.Message }
    if (!$failure -or !$failure.Contains($message)) { throw "Expected rejection containing '$message', received '$failure'." }
}
$savedSettings = @{}
foreach ($name in @('AI_PYTHON','HTTP_ADDR','AI_BASE_URL')) { $savedSettings[$name] = [Environment]::GetEnvironmentVariable($name, 'Process') }
$busy = $null
try {
    $erpPort = Get-TestPort
    do { $aiPort = Get-TestPort } while ($aiPort -eq $erpPort)
    Write-TestConfiguration $erpPort $aiPort
    $output = & $launcher -Check
    if (!$output.Contains("ERP http://127.0.0.1:$erpPort; AI http://127.0.0.1:$aiPort")) { throw 'Configured endpoint URLs were not preserved.' }
    Write-TestConfiguration $erpPort $erpPort
    Expect-Rejection 'different local ports'
    Write-TestConfiguration $erpPort $aiPort '192.0.2.1'
    Expect-Rejection 'plain loopback HTTP endpoints'
    $busy = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
    $busy.Start()
    Write-TestConfiguration $busy.LocalEndpoint.Port $aiPort
    Expect-Rejection 'already in use'
    Write-Output '4 launcher preflight checks passed: custom ports, duplicate ports, non-loopback URL and occupied port.'
} finally {
    if ($busy) { $busy.Stop() }
    foreach ($name in $savedSettings.Keys) { [Environment]::SetEnvironmentVariable($name, $savedSettings[$name], 'Process') }
    Set-Location $previousLocation
    Remove-Item -LiteralPath $launcher, $configFile -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $scriptFolder, $testRoot -ErrorAction SilentlyContinue
}
