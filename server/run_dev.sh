#!/bin/bash
# Runner script to keep the server alive and support restart commands

echo "[Runner] ECPay Server Runner Started"

while true; do
    echo "[Runner] Starting ECPay Server..."
    
    # Run the server and pass all arguments to it
    go run main.go "$@"
    
    EXIT_CODE=$?
    echo "[Runner] Server exited with code $EXIT_CODE"
    
    # If the user killed the runner loop (e.g. via stop.sh sending SIGTERM to this script), we should exit?
    # But this script runs in background in start.sh.
    # We will rely on stop.sh killing the process group or this PID explicitly.
    
    echo "[Runner] Restarting in 1 second..."
    sleep 1
done
