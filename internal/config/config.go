package config

import (
	"flag"
	"fmt"
	"os"
)

type Config struct {
	ServerAddr string // ":8080" or "localhost:8888"
	BaseURL    string // "http://localhost:8080"
}

func New() *Config {
	cfg := &Config{
		ServerAddr: ":8080",
		BaseURL:    "http://localhost:8080",
	}

	flag.StringVar(&cfg.ServerAddr, "a", cfg.ServerAddr, "HTTP-server address (:8080 or localhost:8888)")
	flag.StringVar(&cfg.BaseURL, "b", cfg.BaseURL, "short URL base address (http://localhost:8080)")

	flag.Parse()

	if cfg.ServerAddr == "" {
		fmt.Fprintln(os.Stderr, "Server address can not be empty")
		os.Exit(1)
	}

	// if -b empty
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://" + cfg.ServerAddr
	}

	return cfg
}
