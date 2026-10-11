# SmartNet Windows Installer
# Usage: irm https://raw.githubusercontent.com/lovelystarcc/smartnet/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

Write-Host "Installing SmartNet CLI..."

$arch = if ([System.Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
    Write-Error "32-bit Windows is not supported."
    exit 1
}

$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if ($isAdmin) {
    $installDir = "$env:ProgramFiles\SmartNet"
    $scope = "Machine"
} else {
    $installDir = "$env:LOCALAPPDATA\Programs\SmartNet"
    $scope = "User"
}

if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

$targetBin = Join-Path $installDir "smartnet.exe"
$tmpBin = Join-Path $installDir "smartnet_download_$PID.exe"

Get-Process smartnet -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

$binName = "smartnet-windows-$arch.exe"
$downloadUrl = "https://github.com/lovelystarcc/smartnet/releases/latest/download/$binName"

[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

Write-Host "Downloading $binName from GitHub..."

Invoke-WebRequest -Uri $downloadUrl -OutFile $tmpBin -UseBasicParsing
Move-Item -Path $tmpBin -Destination $targetBin -Force

$currentPath = [System.Environment]::GetEnvironmentVariable("Path", $scope)
if ($currentPath -split ";" -notcontains $installDir) {
    $newPath = "$currentPath;$installDir"
    [System.Environment]::SetEnvironmentVariable("Path", $newPath, $scope)
    $env:Path += ";$installDir"
}

Write-Host "Installed to $targetBin"

try {
    & "$targetBin" version
} catch {}

Write-Host ""
Write-Host "Quick start:"
Write-Host "  smartnet setup <url> [name]"
Write-Host "  smartnet on"
Write-Host "  smartnet test"
Write-Host "  smartnet off"
