#!/bin/bash
# TCB POS System - One-Click Start Script
#
# Architecture:
#   Mock POS (TCP:9999) <---> Server_TCB <---> Webapp
#
# The Server_TCB connects to Mock POS via tcp://localhost:9999 (no ECHO handshake)
# In production, Server_TCB connects to real POS on COM ports

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_DIR="$SCRIPT_DIR/logs"

mkdir -p "$LOG_DIR"

echo "=========================================="
echo "  TCB POS System - Starting Services"
echo "=========================================="

# Kill any existing processes
echo "Cleaning up existing processes..."
pkill -f "mock-pos-tcb" 2>/dev/null || true
pkill -f "tcb-server" 2>/dev/null || true
pkill -f "run_dev.sh" 2>/dev/null || true
lsof -ti:9999 | xargs kill -9 2>/dev/null || true
lsof -ti:8989 | xargs kill -9 2>/dev/null || true
lsof -ti:5173 | xargs kill -9 2>/dev/null || true
sleep 1

# 1. Build & Start Mock POS
echo ""
echo "[1/3] Building & Starting Mock POS (TCB, TCP :9999)..."
cd "$SCRIPT_DIR/mock-pos-tcb"
echo "      Building mock-pos-tcb..."
go build -o mock-pos-tcb
./mock-pos-tcb > "$LOG_DIR/mock-pos.log" 2>&1 &
MOCK_PID=$!
echo $MOCK_PID > "$LOG_DIR/mock-pos.pid"
echo "      Mock POS PID: $MOCK_PID"
sleep 1

if ! kill -0 $MOCK_PID 2>/dev/null; then
    echo "      ✗ ERROR: Mock POS failed to start"
    cat "$LOG_DIR/mock-pos.log" 2>/dev/null | tail -10
    exit 1
fi
echo "      ✓ Mock POS started"

# 2. Build & Start Server_TCB
echo "[2/3] Building & Starting Server_TCB..."
cd "$SCRIPT_DIR/server_tcb"
echo "      Building tcb-server..."
go build -o tcb-server
./run_dev.sh -port tcp://localhost:9999 > "$LOG_DIR/server.log" 2>&1 &
SERVER_PID=$!
echo $SERVER_PID > "$LOG_DIR/server.pid"
echo "      Server Runner PID: $SERVER_PID"
sleep 3

if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo "      ✗ ERROR: Server_TCB failed to start"
    cat "$LOG_DIR/server.log" 2>/dev/null | tail -10
    kill $MOCK_PID 2>/dev/null || true
    exit 1
fi
echo "      ✓ Server_TCB started"

# 3. Install deps (if needed) & Start Webapp
echo "[3/3] Installing deps & Starting Webapp (port 5173)..."
cd "$SCRIPT_DIR/webapp"
npm install > "$LOG_DIR/webapp_npm.log" 2>&1 || true
npm run dev > "$LOG_DIR/webapp.log" 2>&1 &
WEBAPP_PID=$!
echo $WEBAPP_PID > "$LOG_DIR/webapp.pid"
echo "      Webapp PID: $WEBAPP_PID"
sleep 3

if ! kill -0 $WEBAPP_PID 2>/dev/null; then
    echo "      ✗ ERROR: Webapp failed to start"
    cat "$LOG_DIR/webapp.log" 2>/dev/null | tail -10
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
echo "  Mock POS:  tcp://localhost:9999 (PID: $MOCK_PID)"
echo "  Server:    ws://localhost:8989  (PID: $SERVER_PID)"
echo "  Webapp:    http://localhost:5173 (PID: $WEBAPP_PID)"
echo ""
echo "  Logs:      $LOG_DIR/"
echo "  Stop:      ./stop.sh"
echo ""
echo "=========================================="
