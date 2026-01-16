#!/bin/bash
# ECPay POS System - One-Click Stop Script
# Stops: Mock POS, Server, and Webapp

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_DIR="$SCRIPT_DIR/logs"

echo "=========================================="
echo "  ECPay POS System - Stopping Services"
echo "=========================================="

# Stop processes by PID files if they exist
if [ -f "$LOG_DIR/webapp.pid" ]; then
    WEBAPP_PID=$(cat "$LOG_DIR/webapp.pid")
    if kill -0 $WEBAPP_PID 2>/dev/null; then
        echo "Stopping Webapp (PID: $WEBAPP_PID)..."
        kill $WEBAPP_PID 2>/dev/null || true
    fi
    rm -f "$LOG_DIR/webapp.pid"
fi

if [ -f "$LOG_DIR/server.pid" ]; then
    SERVER_PID=$(cat "$LOG_DIR/server.pid")
    if kill -0 $SERVER_PID 2>/dev/null; then
        echo "Stopping Server (PID: $SERVER_PID)..."
        kill $SERVER_PID 2>/dev/null || true
    fi
    rm -f "$LOG_DIR/server.pid"
fi

if [ -f "$LOG_DIR/mock-pos.pid" ]; then
    MOCK_PID=$(cat "$LOG_DIR/mock-pos.pid")
    if kill -0 $MOCK_PID 2>/dev/null; then
        echo "Stopping Mock POS (PID: $MOCK_PID)..."
        kill $MOCK_PID 2>/dev/null || true
    fi
    rm -f "$LOG_DIR/mock-pos.pid"
fi

# Also kill by port (in case PIDs are stale)
echo ""
echo "Cleaning up any remaining processes on ports..."
lsof -ti:5173 | xargs kill -9 2>/dev/null || true
lsof -ti:8989 | xargs kill -9 2>/dev/null || true
lsof -ti:9999 | xargs kill -9 2>/dev/null || true

echo ""
echo "=========================================="
echo "  All Services Stopped"
echo "=========================================="
