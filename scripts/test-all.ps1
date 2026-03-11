Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $PSCommandPath
$rootDir = (Resolve-Path (Join-Path $scriptDir "..")).Path

Write-Host "==> Backend tests"
Push-Location $rootDir
try {
  go test ./...
}
finally {
  Pop-Location
}

Write-Host "==> Frontend install"
Push-Location (Join-Path $rootDir "web")
try {
  npm ci
  Write-Host "==> Frontend unit tests"
  npm run test
  Write-Host "==> Frontend build"
  npm run build
}
finally {
  Pop-Location
}

Write-Host "All tests passed."
