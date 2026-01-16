package config

import (
	"flag"
)

type Config struct {
	WSAddr string // WebSocket server address
}

func Load() *Config {
	wsAddr := flag.String("ws", ":8989", "WebSocket server address")
	flag.Parse()

	return &Config{
		WSAddr: *wsAddr,
	}
}
