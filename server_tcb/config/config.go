package config

import (
	"flag"
)

type Config struct {
	WSAddr     string // WebSocket server address
	SerialPort string // Serial port name (e.g., COM3, /dev/ttyUSB0, tcp://host:port)
	BaudRate   int    // Baud rate, default 115200
	AutoDetect bool   // Enable port auto-detection (not recommended for TCB)
}

func Load() *Config {
	wsAddr := flag.String("ws", ":8989", "WebSocket server address")
	serialPort := flag.String("port", "", "Serial port (COM3, /dev/ttyUSB0, tcp://host:port)")
	baudRate := flag.Int("baud", 115200, "Serial baud rate")
	autoDetect := flag.Bool("autodetect", false, "Auto-detect POS device (TCB does not define ECHO)")
	flag.Parse()

	return &Config{
		WSAddr:     *wsAddr,
		SerialPort: *serialPort,
		BaudRate:   *baudRate,
		AutoDetect: *autoDetect,
	}
}
