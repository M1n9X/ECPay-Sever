package main

import (
	"ecpay-server/api"
	"ecpay-server/config"
	"ecpay-server/driver"
	"ecpay-server/logger"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
)

func main() {
	// 1. Load Config
	cfg := config.Load()

	// 2. Initialize Logger
	logDir := filepath.Join(".", "log")
	if err := logger.Init(logDir); err != nil {
		log.Printf("Warning: Failed to initialize file logger: %v", err)
	}
	defer logger.Close()

	logger.Info("ECPay POS Server starting...")
	fmt.Println("ECPay POS Server starting...")
	fmt.Println("Serial port auto-detection enabled")

	// 3. Initialize Serial Manager with auto-detection
	// Port starts as nil, Scanner will auto-detect and connect
	manager := driver.NewSerialManager(nil)

	// 4. Initialize API Handler
	handler := api.NewHandler(manager)

	// 5. Start HTTP Server
	http.HandleFunc("/ws", handler.ServeWS)

	logger.Info("WebSocket server listening on %s", cfg.WSAddr)
	fmt.Printf("WebSocket server listening on %s\n", cfg.WSAddr)

	if err := http.ListenAndServe(cfg.WSAddr, nil); err != nil {
		logger.Error("ListenAndServe failed: %v", err)
		log.Fatal("ListenAndServe:", err)
	}
}
