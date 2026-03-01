package config

import (
	"flag"
	"fmt"
	"os"
)

type Config struct {
	ServerAddr      string // ":8080" or "localhost:8888"
	BaseURL         string // "http://localhost:8080"
	FileStoragePath string
}

const (
	envServerAddr      = "SERVER_ADDRESS"
	envBaseURL         = "BASE_URL"
	envFileStoragePath = "FILE_STORAGE_PATH"

	defaultServerAddr      = ":8080"
	defaultBaseURL         = "http://localhost:8080"
	defaultFileStoragePath = "./urls.json"
)

func New() *Config {
	cfg := &Config{
		ServerAddr:      defaultServerAddr,
		BaseURL:         defaultBaseURL,
		FileStoragePath: defaultFileStoragePath,
	}

	// parsing flags
	flag.StringVar(&cfg.ServerAddr, "a", cfg.ServerAddr, "HTTP-server address (:8080 or localhost:8888)")
	flag.StringVar(&cfg.BaseURL, "b", cfg.BaseURL, "short URL base address (http://localhost:8080)")
	flag.StringVar(&cfg.FileStoragePath, "f", cfg.FileStoragePath, "URL (JSON) storage file path")
	flag.Parse()

	// parsing env
	if v, ok := os.LookupEnv(envServerAddr); ok && v != "" {
		cfg.ServerAddr = v
	}
	if v, ok := os.LookupEnv(envBaseURL); ok && v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv(envFileStoragePath); v != "" {
		cfg.FileStoragePath = v
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
	fmt.Printf("Using FileStoragePath: %s\n", cfg.FileStoragePath)

	return cfg
}
