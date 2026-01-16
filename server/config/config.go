package config

import (
	"flag"
	"os"
	"runtime"
)

// ConnectionMode defines how to connect to POS device
type ConnectionMode string

const (
	ModeSerial ConnectionMode = "serial" // Real RS232 serial port
	ModeTCP    ConnectionMode = "tcp"    // TCP connection (Serial-over-TCP or network POS)
)

type Config struct {
	// Connection settings
	Mode     ConnectionMode
	Port     string // Serial port name (e.g. COM3) or TCP address (e.g. localhost:9999)
	BaudRate int    // Only used for serial mode

	// Server settings
	WSAddr string
}

func Load() *Config {
	// Default port based on OS
	defaultPort := "/dev/ttyUSB0"
	if runtime.GOOS == "windows" {
		defaultPort = "COM3"
	}

	mode := flag.String("mode", "serial", "Connection mode: 'serial' for RS232, 'tcp' for network")
	port := flag.String("port", defaultPort, "Serial port (e.g. COM3) or TCP address (e.g. localhost:9999)")
	wsAddr := flag.String("ws", ":8989", "WebSocket server address")
	flag.Parse()

	// Allow environment variable override
	if envPort := os.Getenv("ECPAY_POS_PORT"); envPort != "" {
		*port = envPort
	}
	if envMode := os.Getenv("ECPAY_POS_MODE"); envMode != "" {
		*mode = envMode
	}

	return &Config{
		Mode:     ConnectionMode(*mode),
		Port:     *port,
		BaudRate: 115200,
		WSAddr:   *wsAddr,
	}
}
