package config

import (
	"flag"
	"fmt"
	"go-url-shortener/internal/profiler"
	"os"
	"strconv"

	"github.com/google/uuid"
)

type Config struct {
	ServerAddr      string // ":8080" or "localhost:8888"
	BaseURL         string // "http://localhost:8080"
	FileStoragePath string
	DatabaseDSN     string
	CookieSecret    string
	AuditFilePath   string
	AuditURL        string
	EnablePprof     bool
}

const (
	envServerAddr      = "SERVER_ADDRESS"
	envBaseURL         = "BASE_URL"
	envFileStoragePath = "FILE_STORAGE_PATH"
	envDatabaseDSN     = "DATABASE_DSN"
	envCookieSecret    = "COOKIE_SECRET"
	envAuditFile       = "AUDIT_FILE"
	envAuditURL        = "AUDIT_URL"
	envEnablePprof     = "ENABLE_PPROF"

	defaultServerAddr      = ":8080"
	defaultBaseURL         = "http://localhost:8080"
	defaultFileStoragePath = "./urls.txt"
)

func New() *Config {
	cfg := &Config{
		ServerAddr:      defaultServerAddr,
		BaseURL:         defaultBaseURL,
		FileStoragePath: defaultFileStoragePath,
		DatabaseDSN:     "",
		CookieSecret:    "",
		AuditFilePath:   "",
		AuditURL:        "",
		EnablePprof:     false,
	}

	// parsing flags
	flag.StringVar(&cfg.ServerAddr, "a", cfg.ServerAddr, "HTTP-server address (:8080 or localhost:8888)")
	flag.StringVar(&cfg.BaseURL, "b", cfg.BaseURL, "short URL base address (http://localhost:8080)")
	flag.StringVar(&cfg.FileStoragePath, "f", cfg.FileStoragePath, "URL (JSON) storage file path")
	flag.StringVar(&cfg.DatabaseDSN, "d", cfg.DatabaseDSN, "PostgreSQL (DATABASE_DSN)")
	flag.StringVar(&cfg.CookieSecret, "s", cfg.CookieSecret, "Secret key for cookie signing")
	flag.StringVar(&cfg.AuditFilePath, "audit-file", cfg.AuditFilePath, "Audit file path")
	flag.StringVar(&cfg.AuditURL, "audit-url", cfg.AuditURL, "Audit URL address")
	flag.BoolVar(&cfg.EnablePprof, "enable-pprof", false, "Enable debug/pprof on URL address "+profiler.DefaultAddr)
	flag.Parse()

	// parsing env
	if v, ok := os.LookupEnv(envServerAddr); ok {
		cfg.ServerAddr = v
	}
	if v, ok := os.LookupEnv(envBaseURL); ok {
		cfg.BaseURL = v
	}
	if v, ok := os.LookupEnv(envFileStoragePath); ok {
		cfg.FileStoragePath = v
	}
	if v, ok := os.LookupEnv(envDatabaseDSN); ok {
		cfg.DatabaseDSN = v
	}
	if v, ok := os.LookupEnv(envCookieSecret); ok {
		cfg.CookieSecret = v
	}
	if v, ok := os.LookupEnv(envAuditFile); ok {
		cfg.AuditFilePath = v
	}
	if v, ok := os.LookupEnv(envAuditURL); ok {
		cfg.AuditURL = v
	}
	if val, ok := os.LookupEnv(envEnablePprof); ok {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Invalid ENABLE_PPROF value")
			os.Exit(1)
		}
		cfg.EnablePprof = enabled
	}

	if cfg.ServerAddr == "" {
		fmt.Fprintln(os.Stderr, "Server address can not be empty")
		os.Exit(1)
	}

	// if BaseURL (flag -b or env BASE_URL) empty
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://" + cfg.ServerAddr
	}

	if cfg.CookieSecret == "" {
		cfg.CookieSecret = uuid.NewString()
	}

	return cfg
}
