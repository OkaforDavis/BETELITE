@echo off
REM BETELITE Mobile Gaming Platform - Quick Start Script
REM This script helps you run BETELITE on your phone via Docker

echo.
echo ======================================
echo  BETELITE Mobile Platform
echo  Quick Start Menu
echo ======================================
echo.
echo Options:
echo.
echo [1] Start Backend with Docker (Recommended)
echo [2] Stop Backend Services
echo [3] View Backend Logs
echo [4] Show Your Computer IP (for phone access)
echo [5] Open GitHub Pages Demo
echo [6] View Full Setup Guide
echo [7] Exit
echo.

set /p choice="Enter your choice (1-7): "

if "%choice%"=="1" goto start_docker
if "%choice%"=="2" goto stop_docker
if "%choice%"=="3" goto view_logs
if "%choice%"=="4" goto show_ip
if "%choice%"=="5" goto github_pages
if "%choice%"=="6" goto setup_guide
if "%choice%"=="7" goto exit_script
goto invalid

:start_docker
echo.
echo Starting Docker services...
docker-compose up -d
echo.
echo ✓ Services starting...
echo.
echo Backend API: http://localhost:3000
echo PostgreSQL: localhost:5432
echo Redis: localhost:6379
echo AI Detection: http://localhost:5000
echo.
echo Waiting for services to be ready...
timeout /t 5 /nobreak
echo.
echo ✓ Services are running!
echo.
echo Next steps:
echo 1. Get your IP: Run this script again and choose option [4]
echo 2. On your phone, visit: http://YOUR_IP:3000/mobile/
echo.
pause
goto end

:stop_docker
echo.
echo Stopping Docker services...
docker-compose down
echo.
echo ✓ All services stopped
echo.
pause
goto end

:view_logs
echo.
echo Backend logs (Press Ctrl+C to stop):
echo.
docker-compose logs -f backend
goto end

:show_ip
echo.
echo Your Computer IP Addresses:
echo.
ipconfig | findstr /R "IPv4"
echo.
echo Use the WiFi IPv4 address (usually 192.168.x.x or 10.0.x.x)
echo.
echo On your phone, visit:
echo http://[YOUR_IPv4_ADDRESS]:3000/mobile/
echo.
echo Example: http://192.168.1.100:3000/mobile/
echo.
pause
goto end

:github_pages
echo.
echo Opening GitHub Pages demo in browser...
echo.
start https://okafordavis.github.io/BETELITE/mobile/
echo.
echo This version works completely offline with demo data.
echo.
pause
goto end

:setup_guide
echo.
echo Opening MOBILE_SETUP.md...
echo.
start notepad MOBILE_SETUP.md
pause
goto end

:invalid
echo.
echo Invalid choice. Please enter 1-7.
echo.
pause
goto end

:exit_script
echo.
echo Exiting...
exit /b 0

:end
cls
goto start_docker
