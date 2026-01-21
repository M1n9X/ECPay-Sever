#!/bin/bash
# Windows Build Script for ECPay POS
# Usage: ./build-windows.sh [--debug] [--arm64] [--all] [--clean]

set -e

DEBUG=false
ARM64=false
X64=true
CLEAN=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --debug) DEBUG=true; shift ;;
    --arm64) X64=false; ARM64=true; shift ;;
    --all) X64=true; ARM64=true; shift ;;
    --clean) CLEAN=true; shift ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

echo "🔨 ECPay POS Windows Build"
echo "=========================="

# Clean
if [ "$CLEAN" = true ]; then
  echo "🧹 Cleaning..."
  rm -rf release dist
fi

# Build Go server
echo "📦 Building Go server..."
npm run build:go:win

# Build TypeScript
echo "🏗️  Building TypeScript..."
npm run build

# Build Windows installer
echo "📦 Building Windows installer..."

export CSC_IDENTITY_AUTO_DISCOVERY=false

if [ "$X64" = true ]; then
  echo "🔨 Building x64..."
  npx electron-builder --win --x64 -p never
fi

if [ "$ARM64" = true ]; then
  echo "🔨 Building ARM64..."
  npx electron-builder --win --arm64 -p never
fi

echo ""
echo "✅ Build Complete!"
echo "📁 Output: release/"
ls -lh release/*.exe 2>/dev/null | awk '{print "   " $9 " (" $5 ")"}'
