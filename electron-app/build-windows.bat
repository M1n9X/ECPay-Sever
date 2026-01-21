@echo off
REM Windows Build Script for ECPay POS
REM Usage: build-windows.bat [--debug] [--arm64] [--all] [--clean]

setlocal enabledelayedexpansion

set DEBUG=false
set ARM64=false
set X64=true
set CLEAN=false

:parse_args
if "%1"=="" goto args_done
if "%1"=="--debug" (set DEBUG=true & shift & goto parse_args)
if "%1"=="--arm64" (set X64=false & set ARM64=true & shift & goto parse_args)
if "%1"=="--all" (set X64=true & set ARM64=true & shift & goto parse_args)
if "%1"=="--clean" (set CLEAN=true & shift & goto parse_args)
echo Unknown option: %1
exit /b 1

:args_done
echo.
echo 🔨 ECPay POS Windows Build
echo ==========================

REM Clean
if "%CLEAN%"=="true" (
  echo 🧹 Cleaning...
  if exist release rmdir /s /q release
  if exist dist rmdir /s /q dist
)

REM Build Go server
echo 📦 Building Go server...
call npm run build:go:win
if errorlevel 1 exit /b 1

REM Build TypeScript
echo 🏗️  Building TypeScript...
call npm run build
if errorlevel 1 exit /b 1

REM Build Windows installer
echo 📦 Building Windows installer...
set CSC_IDENTITY_AUTO_DISCOVERY=false

if "%X64%"=="true" (
  echo 🔨 Building x64...
  call npx electron-builder --win --x64 -p never
  if errorlevel 1 exit /b 1
)

if "%ARM64%"=="true" (
  echo 🔨 Building ARM64...
  call npx electron-builder --win --arm64 -p never
  if errorlevel 1 exit /b 1
)

echo.
echo ✅ Build Complete!
echo 📁 Output: release\
for %%F in (release\*.exe) do echo    %%~nxF
