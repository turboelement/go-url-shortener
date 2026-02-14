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

const (
	envServerAddr = "SERVER_ADDRESS"
	envBaseURL    = "BASE_URL"
)

func New() *Config {
	cfg := &Config{
		ServerAddr: ":8080",
		BaseURL:    "http://localhost:8080",
	}

	// parsing flags
	flag.StringVar(&cfg.ServerAddr, "a", cfg.ServerAddr, "HTTP-server address (:8080 or localhost:8888)")
	flag.StringVar(&cfg.BaseURL, "b", cfg.BaseURL, "short URL base address (http://localhost:8080)")
	flag.Parse()

	// parsing env
	if v, ok := os.LookupEnv(envServerAddr); ok && v != "" {
		cfg.ServerAddr = v
	}
	if v, ok := os.LookupEnv(envBaseURL); ok && v != "" {
		cfg.BaseURL = v
	}

	if cfg.ServerAddr == "" {
		fmt.Fprintln(os.Stderr, "Server address can not be empty")
		os.Exit(1)
	}

	// if BaseURL (flag -b or env BASE_URL) empty
	wasBaseEmpty := cfg.BaseURL == ""
	if wasBaseEmpty {
		cfg.BaseURL = "http://" + cfg.ServerAddr
	}

	fmt.Printf("Using ServerAddr: %s\n", cfg.ServerAddr)
	fmt.Printf("Using BaseURL: %s %s\n",
		cfg.BaseURL,
		map[bool]string{true: "(generated from ServerAddr)", false: ""}[wasBaseEmpty],
	)

	return cfg
}
