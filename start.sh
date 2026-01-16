#!/bin/bash
# ECPay POS System - One-Click Start Script
# Starts: Mock POS, Server, and Webapp

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_DIR="$SCRIPT_DIR/logs"
mkdir -p "$LOG_DIR"

echo "=========================================="
echo "  ECPay POS System - Starting Services"
echo "=========================================="

# Kill any existing processes on our ports
echo "Cleaning up existing processes..."
lsof -ti:9999 | xargs kill -9 2>/dev/null || true
lsof -ti:8989 | xargs kill -9 2>/dev/null || true
lsof -ti:5173 | xargs kill -9 2>/dev/null || true
sleep 1

# Start Mock POS
echo ""
echo "[1/3] Starting Mock POS (port 9999)..."
cd "$SCRIPT_DIR/mock-pos"
go run main.go > "$LOG_DIR/mock-pos.log" 2>&1 &
MOCK_PID=$!
echo "      Mock POS PID: $MOCK_PID"
sleep 1

# Check if mock-pos started
if ! kill -0 $MOCK_PID 2>/dev/null; then
    echo "      ERROR: Mock POS failed to start. Check $LOG_DIR/mock-pos.log"
    exit 1
fi

# Start Server
echo "[2/3] Starting Server in Mock Mode (port 8989)..."
cd "$SCRIPT_DIR/server"
# Use run_dev.sh to support restart
./run_dev.sh -mock > "$LOG_DIR/server.log" 2>&1 &
SERVER_PID=$!
echo "      Server Runner PID: $SERVER_PID"
sleep 2

# Check if server started
if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo "      ERROR: Server failed to start. Check $LOG_DIR/server.log"
    kill $MOCK_PID 2>/dev/null || true
    exit 1
fi

# Start Webapp
echo "[3/3] Starting Webapp (port 5173)..."
cd "$SCRIPT_DIR/webapp"
npm run dev > "$LOG_DIR/webapp.log" 2>&1 &
WEBAPP_PID=$!
echo "      Webapp PID: $WEBAPP_PID"
sleep 3

# Check if webapp started
if ! kill -0 $WEBAPP_PID 2>/dev/null; then
    echo "      ERROR: Webapp failed to start. Check $LOG_DIR/webapp.log"
    kill $MOCK_PID 2>/dev/null || true
    kill $SERVER_PID 2>/dev/null || true
    exit 1
fi

# Save PIDs for stop script
echo "$MOCK_PID" > "$LOG_DIR/mock-pos.pid"
echo "$SERVER_PID" > "$LOG_DIR/server.pid"
echo "$WEBAPP_PID" > "$LOG_DIR/webapp.pid"

echo ""
echo "=========================================="
echo "  All Services Started Successfully!"
echo "=========================================="
echo ""
echo "  Mock POS:  localhost:9999 (PID: $MOCK_PID)"
echo "  Server:    localhost:8989 (PID: $SERVER_PID)"
echo "  Webapp:    http://localhost:5173 (PID: $WEBAPP_PID)"
echo ""
echo "  Logs:      $LOG_DIR/"
echo "  Stop:      ./stop.sh"
echo ""
echo "=========================================="
