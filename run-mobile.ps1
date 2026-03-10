# BETELITE Mobile Quick Start Script
# Usage: .\run-mobile.ps1 [docker|backend|ip]

param(
    [string]$mode = "docker",
    [string]$action = ""
)

function Get-LocalIP {
    $ipConfig = ipconfig | Select-String "IPv4 Address"
    if ($ipConfig) {
        $ip = $ipConfig -split ': ' | Select-Object -Last 1
        return $ip.Trim()
    }
    return "127.0.0.1"
}

function Show-Banner {
    Write-Host "
    ╔════════════════════════════════════════════════════╗
    ║          ⚽ BETELITE MOBILE - Quick Start ⚽        ║
    ╚════════════════════════════════════════════════════╝
    " -ForegroundColor Cyan
}

function Show-Menu {
    Write-Host "`nSelect how to run BETELITE:" -ForegroundColor Yellow
    Write-Host "1. Docker (Full Stack - Recommended)" -ForegroundColor Green
    Write-Host "2. Backend Only (Node.js locally)" -ForegroundColor Green
    Write-Host "3. GitHub Pages (Instant - Demo)" -ForegroundColor Green
    Write-Host "4. Show IP Address" -ForegroundColor Green
    Write-Host "5. Exit" -ForegroundColor Red
    
    $choice = Read-Host "`nEnter choice (1-5)"
    return $choice
}

function Start-Docker {
    Write-Host "`nStarting Docker services..." -ForegroundColor Green
    $ip = Get-LocalIP
    
    Write-Host "
    📱 IMPORTANT: Your Computer IP is: $ip
    
    Once Docker starts, access mobile app from your phone:
    http://$ip:3000/mobile
    " -ForegroundColor Cyan
    
    Write-Host "Starting services..." -ForegroundColor Yellow
    docker-compose up --build
}

function Start-Backend {
    Write-Host "`nStarting Backend Server..." -ForegroundColor Green
    $ip = Get-LocalIP
    
    Write-Host "
    📱 Your Computer IP is: $ip
    
    Access from your phone:
    http://$ip:3000/mobile
    
    Make sure PostgreSQL and Redis are running locally!
    " -ForegroundColor Cyan
    
    Push-Location backend
    Write-Host "Installing dependencies..." -ForegroundColor Yellow
    npm install
    Write-Host "Starting server..." -ForegroundColor Green
    npm run dev
    Pop-Location
}

function Show-GitHub {
    Write-Host "`n
    ╔═══════════════════════════════════════════════════╗
    ║       BETELITE on GitHub Pages - Instant Play    ║
    ╚═══════════════════════════════════════════════════╝
    
    🌐 Visit: https://okafordavis.github.io/BETELITE/
    
    ✅ No setup required
    ✅ Demo mode with full functionality
    ✅ Tournaments, betting, spectating included
    ✅ Works on any phone with a browser
    
    Click 'Launch App' to start playing!
    " -ForegroundColor Green
    
    Start-Process "https://okafordavis.github.io/BETELITE/"
}

function Show-IP {
    $ip = Get-LocalIP
    Write-Host "`n
    ╔═══════════════════════════════════════════════════╗
    ║           Your Computer IP Address                ║
    ╚═══════════════════════════════════════════════════╝
    
    📱 Local IP: $ip
    
    Use this to access from phone:
    http://$ip:3000/mobile
    
    Make sure phone is on SAME WiFi network!
    " -ForegroundColor Cyan
    
    Read-Host "Press Enter to continue"
}

# Main Script
Show-Banner

if ($mode -eq "docker") {
    Start-Docker
} elseif ($mode -eq "backend") {
    Start-Backend
} elseif ($mode -eq "ip") {
    Show-IP
} else {
    # Show interactive menu
    do {
        $choice = Show-Menu
        
        switch ($choice) {
            "1" { Start-Docker; break }
            "2" { Start-Backend; break }
            "3" { Show-GitHub }
            "4" { Show-IP }
            "5" { Write-Host "Goodbye!`n" -ForegroundColor Yellow; exit }
            default { Write-Host "Invalid choice. Try again." -ForegroundColor Red }
        }
    } while ($choice -ne "5")
}
