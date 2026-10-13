$ErrorActionPreference = 'Stop'
$stateFile = Join-Path (Split-Path $PSScriptRoot -Parent) 'var/processes.json'
if (!(Test-Path -LiteralPath $stateFile)) { Write-Output 'No recorded local services.'; return }
function Stop-ServiceTree([int]$serviceId) {
    foreach ($child in @(Get-CimInstance Win32_Process -Filter "ParentProcessId=$serviceId")) {
        Stop-ServiceTree $child.ProcessId
    }
    Stop-Process -Id $serviceId -ErrorAction SilentlyContinue
}
foreach ($record in @(Get-Content -LiteralPath $stateFile -Raw | ConvertFrom-Json)) {
    $service = Get-Process -Id $record.Id -ErrorAction SilentlyContinue
    if ($service -and $service.StartTime.ToUniversalTime().Ticks -eq $record.StartTicks) {
        Stop-ServiceTree $service.Id
    }
}
Remove-Item -LiteralPath $stateFile
Write-Output 'Local application services stopped. PostgreSQL remains running.'

