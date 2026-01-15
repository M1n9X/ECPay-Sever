package config

import "flag"

type Config struct {
	Port     string
	BaudRate int
	Mock     bool
}

func Load() *Config {
	port := flag.String("port", "COM3", "Serial Port Name (e.g. COM3 or /dev/ttyUSB0)")
	mock := flag.Bool("mock", false, "Enable Mock Mode (Simulate POS)")
	flag.Parse()

	return &Config{
		Port:     *port,
		BaudRate: 115200,
		Mock:     *mock,
	}
}
