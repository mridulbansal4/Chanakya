# dev.ps1 — start both CHANAKYA services for local development (Windows).
#
#   .\dev.ps1
#
# Starts the Go backend (http://localhost:8080) and the Next.js web app
# (http://localhost:3000) in two child windows. No Docker, no external DB —
# the backend creates ./chanakya.db on first run AND self-seeds the SEBI IA
# circular (seeds + compiles the fixture), so the app is fully working with no
# manual seed step.
#
# Requires: Go 1.24+ and Node 20+ on PATH.

$ErrorActionPreference = "Stop"
$root = $PSScriptRoot

# Refresh PATH from the registry so a freshly-installed Go is picked up even in
# a shell that was open before installation.
$env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" +
            [System.Environment]::GetEnvironmentVariable("Path", "User")

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go not found on PATH. Install with: winget install --id GoLang.Go -e"
}
if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
    throw "Node not found on PATH. Install Node 20+."
}

Write-Host "CHANAKYA dev — starting backend + web..." -ForegroundColor Cyan

# Backend: go run ./backend/cmd/api  (uses go.work at repo root)
Start-Process powershell -ArgumentList "-NoExit", "-Command", "go run ./backend/cmd/api" -WorkingDirectory $root

# Web: npm run dev  (turbo dev → next dev on :3000) — runs from ./frontend
Start-Process powershell -ArgumentList "-NoExit", "-Command", "npm run dev" -WorkingDirectory "$root\frontend"

Write-Host "Backend:  http://localhost:8080/health" -ForegroundColor Yellow
Write-Host "Web app:  http://localhost:3000" -ForegroundColor Yellow
Write-Host "Close the two spawned windows to stop." -ForegroundColor Yellow
