package config

import (
	"flag"
	"os"
	"runtime"
)

type Config struct {
	Port     string // Serial port name (e.g. COM3, /dev/ttyUSB0, /tmp/mock-pos-pty)
	BaudRate int
	WSAddr   string
}

func Load() *Config {
	// Default port based on OS
	defaultPort := "/dev/ttyUSB0"
	if runtime.GOOS == "windows" {
		defaultPort = "COM3"
	}

	port := flag.String("port", defaultPort, "Serial port (e.g. COM3, /dev/ttyUSB0)")
	wsAddr := flag.String("ws", ":8989", "WebSocket server address")
	flag.Parse()

	// Allow environment variable override
	if envPort := os.Getenv("ECPAY_SERIAL_PORT"); envPort != "" {
		*port = envPort
	}

	return &Config{
		Port:     *port,
		BaudRate: 115200,
		WSAddr:   *wsAddr,
	}
}
