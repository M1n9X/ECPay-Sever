package main

import (
	"tcb-server/api"
	"tcb-server/config"
	"tcb-server/driver"
	"tcb-server/logger"
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

	logger.Info("TCB POS Server starting...")
	fmt.Println("TCB POS Server starting...")

	// 3. Initialize Serial Manager
	var manager *driver.SerialManager
	if cfg.SerialPort != "" {
		port, err := driver.OpenSerial(cfg.SerialPort, cfg.BaudRate)
		if err != nil {
			logger.Error("Failed to open serial port %s: %v", cfg.SerialPort, err)
			fmt.Printf("Failed to open serial port %s: %v\n", cfg.SerialPort, err)
			manager = driver.NewSerialManager(nil, cfg.BaudRate, cfg.AutoDetect)
		} else {
			manager = driver.NewSerialManager(port, cfg.BaudRate, cfg.AutoDetect)
			fmt.Printf("Connected to %s at %d bps\n", cfg.SerialPort, cfg.BaudRate)
		}
	} else {
		manager = driver.NewSerialManager(nil, cfg.BaudRate, cfg.AutoDetect)
		if cfg.AutoDetect {
			fmt.Println("Serial port auto-detection enabled")
		} else {
			fmt.Println("No serial port specified. Use -port to connect.")
		}
	}

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
