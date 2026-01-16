package main

import (
	"ecpay-server/api"
	"ecpay-server/config"
	"ecpay-server/driver"
	"fmt"
	"log"
	"net/http"
)

func main() {
	// 1. Load Config
	cfg := config.Load()

	// 2. Initialize Port (Serial or TCP)
	var port driver.Port
	var err error

	if cfg.Mock {
		fmt.Println("Starting in MOCK MODE (connecting to Mock POS at localhost:9999)")
		port, err = driver.OpenTCP("localhost:9999")
		if err != nil {
			log.Fatalf("Failed to connect to Mock POS: %v", err)
		}
	} else {
		fmt.Printf("Opening Serial Port: %s\n", cfg.Port)
		port, err = driver.OpenSerial(cfg.Port, cfg.BaudRate)
		if err != nil {
			log.Fatalf("Failed to open serial port: %v", err)
		}
	}
	defer port.Close()

	// 3. Initialize Manager
	manager := driver.NewSerialManager(port)

	// 4. Initialize API Handler
	handler := api.NewHandler(manager)

	// 5. Start HTTP Server
	http.HandleFunc("/ws", handler.ServeWS)

	addr := ":8989"
	fmt.Printf("Server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
