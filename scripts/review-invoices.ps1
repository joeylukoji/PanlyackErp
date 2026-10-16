param(
    [string]$Dataset = '',
    [string]$Predictions = '',
    [ValidateRange(1024, 65535)][int]$Port = 8091
)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
if (-not $Dataset) { $Dataset = Join-Path $projectRoot 'var/evaluations/invoices-100' }
if (-not $Predictions) { $Predictions = Join-Path $Dataset 'runs/date-guidance/predictions.json' }
$datasetPath = (Resolve-Path -LiteralPath $Dataset).Path
$predictionsPath = (Resolve-Path -LiteralPath $Predictions).Path
$pythonPath = Join-Path $projectRoot '.venv/Scripts/python.exe'
if (-not (Test-Path -LiteralPath $pythonPath)) { throw 'The project Python environment is missing.' }
Push-Location (Join-Path $projectRoot 'ai-service')
try {
    & $pythonPath -m app.annotation_review --dataset $datasetPath --predictions $predictionsPath --port $Port
    if ($LASTEXITCODE -ne 0) { throw "Annotation review stopped with exit code $LASTEXITCODE." }
} finally {
    Pop-Location
}
