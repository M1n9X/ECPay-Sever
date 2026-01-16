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

	// 3. Initialize Port (Serial or TCP)
	var port driver.Port
	var err error

	if cfg.Mock {
		logger.Info("Starting in MOCK MODE (connecting to Mock POS at localhost:9999)")
		fmt.Println("Starting in MOCK MODE (connecting to Mock POS at localhost:9999)")
		port, err = driver.OpenTCP("localhost:9999")
		if err != nil {
			logger.Error("Failed to connect to Mock POS: %v", err)
			log.Fatalf("Failed to connect to Mock POS: %v", err)
		}
	} else {
		logger.Info("Opening Serial Port: %s", cfg.Port)
		fmt.Printf("Opening Serial Port: %s\n", cfg.Port)
		port, err = driver.OpenSerial(cfg.Port, cfg.BaudRate)
		if err != nil {
			logger.Error("Failed to open serial port: %v", err)
			log.Fatalf("Failed to open serial port: %v", err)
		}
	}
	defer port.Close()

	// 4. Initialize Manager
	manager := driver.NewSerialManager(port)

	// 5. Initialize API Handler
	handler := api.NewHandler(manager)

	// 6. Start HTTP Server
	http.HandleFunc("/ws", handler.ServeWS)

	addr := ":8989"
	logger.Info("Server listening on %s", addr)
	fmt.Printf("Server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Error("ListenAndServe failed: %v", err)
		log.Fatal("ListenAndServe:", err)
	}
}
