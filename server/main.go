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

	// 3. Initialize Port based on mode
	var port driver.Port
	var err error

	switch cfg.Mode {
	case config.ModeTCP:
		// TCP mode: connect directly to specified address
		logger.Info("Mode: TCP (connecting to %s)", cfg.Port)
		fmt.Printf("Mode: TCP (connecting to %s)\n", cfg.Port)
		port, err = driver.OpenTCP(cfg.Port)
		if err != nil {
			logger.Error("Failed to connect: %v", err)
			log.Fatalf("Failed to connect to %s: %v", cfg.Port, err)
		}

	case config.ModeSerial:
		fallthrough
	default:
		// Serial mode: use auto-detection scanner
		logger.Info("Mode: Serial (auto-detection enabled)")
		fmt.Println("Mode: Serial (auto-detection enabled)")
		// Port remains nil, Manager will start scanner
	}

	if port != nil {
		defer port.Close()
	}

	// 4. Initialize Manager
	manager := driver.NewSerialManager(port)

	// 5. Initialize API Handler
	handler := api.NewHandler(manager)

	// 6. Start HTTP Server
	http.HandleFunc("/ws", handler.ServeWS)

	logger.Info("WebSocket server listening on %s", cfg.WSAddr)
	fmt.Printf("WebSocket server listening on %s\n", cfg.WSAddr)

	if err := http.ListenAndServe(cfg.WSAddr, nil); err != nil {
		logger.Error("ListenAndServe failed: %v", err)
		log.Fatal("ListenAndServe:", err)
	}
}
