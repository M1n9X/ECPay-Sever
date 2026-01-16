#!/bin/bash
# ECPay POS System - One-Click Start Script
#
# Architecture:
#   Mock POS (PTY) <--serial--> Server <--websocket--> Webapp
#
# The Mock POS creates a virtual serial port (/tmp/mock-pos-pty)
# Server auto-detects this port and performs ECHO handshake
# This makes development environment identical to production

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_DIR="$SCRIPT_DIR/logs"
PTY_LINK="/tmp/mock-pos-pty"

mkdir -p "$LOG_DIR"

echo "=========================================="
echo "  ECPay POS System - Starting Services"
echo "=========================================="

# Check for socat (required for PTY mode)
if ! command -v socat &> /dev/null; then
    echo ""
    echo "ERROR: 'socat' is required but not found."
    echo ""
    echo "Install with:"
    echo "  macOS:  brew install socat"
    echo "  Ubuntu: sudo apt install socat"
    echo "  CentOS: sudo yum install socat"
    echo ""
    exit 1
fi

# Kill any existing processes
echo "Cleaning up existing processes..."
pkill -f "mock-pos" 2>/dev/null || true
pkill -f "ecpay-server" 2>/dev/null || true
pkill -f "run_dev.sh" 2>/dev/null || true
lsof -ti:8989 | xargs kill -9 2>/dev/null || true
lsof -ti:5173 | xargs kill -9 2>/dev/null || true
rm -f "$PTY_LINK" 2>/dev/null || true
sleep 1

# 1. Start Mock POS (PTY mode - creates virtual serial port)
echo ""
echo "[1/3] Starting Mock POS (PTY mode)..."
cd "$SCRIPT_DIR/mock-pos"
./mock-pos -mode pty -pty-link "$PTY_LINK" > "$LOG_DIR/mock-pos.log" 2>&1 &
MOCK_PID=$!
echo $MOCK_PID > "$LOG_DIR/mock-pos.pid"
echo "      Mock POS PID: $MOCK_PID"

# Wait for PTY to be created
echo "      Waiting for virtual serial port..."
for i in {1..20}; do
    if [ -e "$PTY_LINK" ]; then
        echo "      ✓ Virtual serial port ready: $PTY_LINK"
        break
    fi
    sleep 0.5
done

if [ ! -e "$PTY_LINK" ]; then
    echo "      ✗ ERROR: PTY not created after 10s"
    echo "      Check logs: $LOG_DIR/mock-pos.log"
    cat "$LOG_DIR/mock-pos.log" 2>/dev/null | tail -20
    kill $MOCK_PID 2>/dev/null || true
    exit 1
fi

# 2. Start Server (auto-detects serial port via ECHO handshake)
echo "[2/3] Starting Server (auto-detect mode)..."
cd "$SCRIPT_DIR/server"
./run_dev.sh > "$LOG_DIR/server.log" 2>&1 &
SERVER_PID=$!
echo $SERVER_PID > "$LOG_DIR/server.pid"
echo "      Server Runner PID: $SERVER_PID"
sleep 3

if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo "      ✗ ERROR: Server failed to start"
    echo "      Check logs: $LOG_DIR/server.log"
    kill $MOCK_PID 2>/dev/null || true
    exit 1
fi
echo "      ✓ Server started"

# 3. Start Webapp
echo "[3/3] Starting Webapp (port 5173)..."
cd "$SCRIPT_DIR/webapp"
npm run dev > "$LOG_DIR/webapp.log" 2>&1 &
WEBAPP_PID=$!
echo $WEBAPP_PID > "$LOG_DIR/webapp.pid"
echo "      Webapp PID: $WEBAPP_PID"
sleep 3

if ! kill -0 $WEBAPP_PID 2>/dev/null; then
    echo "      ✗ ERROR: Webapp failed to start"
    echo "      Check logs: $LOG_DIR/webapp.log"
    kill $MOCK_PID 2>/dev/null || true
    kill $SERVER_PID 2>/dev/null || true
    exit 1
fi
echo "      ✓ Webapp started"

echo ""
echo "=========================================="
echo "  All Services Started Successfully!"
echo "=========================================="
echo ""
echo "  Mock POS:  $PTY_LINK (PID: $MOCK_PID)"
echo "  Server:    ws://localhost:8989 (PID: $SERVER_PID)"
echo "  Webapp:    http://localhost:5173 (PID: $WEBAPP_PID)"
echo ""
echo "  Logs:      $LOG_DIR/"
echo "  Stop:      ./stop.sh"
echo ""
echo "=========================================="
