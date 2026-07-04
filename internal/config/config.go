// Package config loads and stores application configuration from JSON config file,
// environment variables, and command-line flags.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"go-url-shortener/internal/profiler"
)

// Config stores application configuration settings.
// generate:reset
type Config struct {
	Server    ServerConfig
	Storage   StorageConfig
	Security  SecurityConfig
	Audit     AuditConfig
	Profiling ProfilingConfig
}

// ServerConfig stores network settings for the HTTP and gRPC servers.
type ServerConfig struct {
	Address       string
	BaseURL       string
	EnableHTTPS   bool
	TrustedSubnet string
	GRPCAddress   string
}

// StorageConfig stores storage backend settings.
type StorageConfig struct {
	FileStoragePath string
	DatabaseDSN     string
}

// SecurityConfig stores security-related settings.
type SecurityConfig struct {
	CookieSecret string
}

// AuditConfig stores audit settings.
type AuditConfig struct {
	FilePath string
	URL      string
}

// ProfilingConfig stores profiling / debug settings.
type ProfilingConfig struct {
	EnablePprof bool
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
	envEnableHTTPS     = "ENABLE_HTTPS"
	envTrustedSubnet   = "TRUSTED_SUBNET"
	envGRPCAddress     = "GRPC_ADDRESS"
	envConfig          = "CONFIG"

	defaultServerAddr      = ":8080"
	defaultGRPCAddr        = ":3200"
	defaultBaseURL         = "http://localhost:8080"
	defaultFileStoragePath = "./urls.txt"
)

var (
	flagServerAddr      = flag.String("a", defaultServerAddr, "HTTP-server address (:8080 or localhost:8888)")
	flagBaseURL         = flag.String("b", defaultBaseURL, "Short URL base address (http://localhost:8080)")
	flagFileStoragePath = flag.String("f", defaultFileStoragePath, "URL (JSON) storage file path")
	flagDatabaseDSN     = flag.String("d", "", "PostgreSQL DSN")
	flagCookieSecret    = flag.String("secret-key", "", "Secret key for cookie signing")
	flagAuditFile       = flag.String("audit-file", "", "Audit file path")
	flagAuditURL        = flag.String("audit-url", "", "Audit URL address")
	flagEnablePprof     = flag.Bool("enable-pprof", false, "Enable debug/pprof on URL address "+profiler.DefaultAddr)
	flagEnableHTTPS     = flag.Bool("s", false, "Enable HTTPS server (self-signed certificate)")
	flagTrustedSubnet   = flag.String("t", "", "Trusted subnet CIDR")
	flagGRPCAddress     = flag.String("g", defaultGRPCAddr, "gRPC-server address (:3200 or localhost:3200)")
	flagConfigPath      = flag.String("c", "", "Path to JSON config file")
	flagConfigPathLong  = flag.String("config", "", "Path to JSON config file")
)

// New reads flags, JSON config file, and environment variables,
// then returns a fully populated Config.
func New() (*Config, error) {
	if !flag.Parsed() {
		flag.Parse()
	}

	visitedFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		visitedFlags[f.Name] = true
	})

	configPath := configPathFromFlags(visitedFlags)
	if v, ok := os.LookupEnv(envConfig); ok {
		configPath = v
	}

	cfg := defaultConfig()
	if configPath != "" {
		if err := cfg.applyConfigFile(configPath); err != nil {
			return nil, fmt.Errorf("config file error: %w", err)
		}
	}

	cfg.applyFlags(visitedFlags)
	cfg.applyEnv()
	cfg.postProcess()

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Address:     defaultServerAddr,
			BaseURL:     defaultBaseURL,
			GRPCAddress: defaultGRPCAddr,
		},
		Storage: StorageConfig{
			FileStoragePath: defaultFileStoragePath,
		},
	}
}

type jsonFile struct {
	ServerAddress   *string `json:"server_address"`
	BaseURL         *string `json:"base_url"`
	FileStoragePath *string `json:"file_storage_path"`
	DatabaseDSN     *string `json:"database_dsn"`
	EnableHTTPS     *bool   `json:"enable_https"`
	CookieSecret    *string `json:"cookie_secret"`
	AuditFilePath   *string `json:"audit_file_path"`
	AuditURL        *string `json:"audit_url"`
	EnablePprof     *bool   `json:"enable_pprof"`
	TrustedSubnet   *string `json:"trusted_subnet"`
	GRPCAddress     *string `json:"grpc_address"`
}

func (cfg *Config) applyConfigFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %q: %w", path, err)
	}

	var jf jsonFile
	if err := json.Unmarshal(data, &jf); err != nil {
		return fmt.Errorf("failed to parse %q: %w", path, err)
	}

	setStr := func(dst *string, src *string) {
		if src != nil {
			*dst = *src
		}
	}
	setBool := func(dst *bool, src *bool) {
		if src != nil {
			*dst = *src
		}
	}

	setStr(&cfg.Server.Address, jf.ServerAddress)
	setStr(&cfg.Server.BaseURL, jf.BaseURL)
	setBool(&cfg.Server.EnableHTTPS, jf.EnableHTTPS)
	setStr(&cfg.Storage.FileStoragePath, jf.FileStoragePath)
	setStr(&cfg.Storage.DatabaseDSN, jf.DatabaseDSN)
	setStr(&cfg.Security.CookieSecret, jf.CookieSecret)
	setStr(&cfg.Audit.FilePath, jf.AuditFilePath)
	setStr(&cfg.Audit.URL, jf.AuditURL)
	setBool(&cfg.Profiling.EnablePprof, jf.EnablePprof)
	setStr(&cfg.Server.TrustedSubnet, jf.TrustedSubnet)
	setStr(&cfg.Server.GRPCAddress, jf.GRPCAddress)

	return nil
}

func configPathFromFlags(visitedFlags map[string]bool) string {
	switch {
	case visitedFlags["c"]:
		return *flagConfigPath
	case visitedFlags["config"]:
		return *flagConfigPathLong
	default:
		return ""
	}
}

func (cfg *Config) applyFlags(visitedFlags map[string]bool) {
	if visitedFlags["a"] {
		cfg.Server.Address = *flagServerAddr
	}
	if visitedFlags["b"] {
		cfg.Server.BaseURL = *flagBaseURL
	}
	if visitedFlags["f"] {
		cfg.Storage.FileStoragePath = *flagFileStoragePath
	}
	if visitedFlags["d"] {
		cfg.Storage.DatabaseDSN = *flagDatabaseDSN
	}
	if visitedFlags["secret-key"] {
		cfg.Security.CookieSecret = *flagCookieSecret
	}
	if visitedFlags["audit-file"] {
		cfg.Audit.FilePath = *flagAuditFile
	}
	if visitedFlags["audit-url"] {
		cfg.Audit.URL = *flagAuditURL
	}
	if visitedFlags["enable-pprof"] {
		cfg.Profiling.EnablePprof = *flagEnablePprof
	}
	if visitedFlags["s"] {
		cfg.Server.EnableHTTPS = *flagEnableHTTPS
	}
	if visitedFlags["t"] {
		cfg.Server.TrustedSubnet = *flagTrustedSubnet
	}
	if visitedFlags["g"] {
		cfg.Server.GRPCAddress = *flagGRPCAddress
	}
}

func (cfg *Config) applyEnv() {
	if v, ok := os.LookupEnv(envServerAddr); ok {
		cfg.Server.Address = v
	}
	if v, ok := os.LookupEnv(envBaseURL); ok {
		cfg.Server.BaseURL = v
	}
	if v, ok := os.LookupEnv(envFileStoragePath); ok {
		cfg.Storage.FileStoragePath = v
	}
	if v, ok := os.LookupEnv(envDatabaseDSN); ok {
		cfg.Storage.DatabaseDSN = v
	}
	if v, ok := os.LookupEnv(envCookieSecret); ok {
		cfg.Security.CookieSecret = v
	}
	if v, ok := os.LookupEnv(envAuditFile); ok {
		cfg.Audit.FilePath = v
	}
	if v, ok := os.LookupEnv(envAuditURL); ok {
		cfg.Audit.URL = v
	}
	if val, ok := os.LookupEnv(envEnablePprof); ok {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid %s value %q, using default (false)\n", envEnablePprof, val)
		} else {
			cfg.Profiling.EnablePprof = enabled
		}
	}
	if val, ok := os.LookupEnv(envEnableHTTPS); ok {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid %s value %q, using default (false)\n", envEnableHTTPS, val)
		} else {
			cfg.Server.EnableHTTPS = enabled
		}
	}
	if v, ok := os.LookupEnv(envTrustedSubnet); ok {
		cfg.Server.TrustedSubnet = v
	}
	if v, ok := os.LookupEnv(envGRPCAddress); ok {
		cfg.Server.GRPCAddress = v
	}
}

func (cfg *Config) postProcess() {
	// Ensure server address is non-empty.
	if cfg.Server.Address == "" {
		fmt.Fprintln(os.Stderr, "Server address cannot be empty, using default :8080")
		cfg.Server.Address = ":8080"
	}

	// Derive BaseURL from address if it was left empty.
	if cfg.Server.BaseURL == "" {
		scheme := "http"
		if cfg.Server.EnableHTTPS {
			scheme = "https"
		}
		cfg.Server.BaseURL = scheme + "://" + cfg.Server.Address
	}

	// Switch to https scheme when HTTPS is enabled.
	if cfg.Server.EnableHTTPS && strings.HasPrefix(cfg.Server.BaseURL, "http://") {
		cfg.Server.BaseURL = "https://" + cfg.Server.BaseURL[len("http://"):]
	}

	// Generate a random cookie secret if none was provided.
	if cfg.Security.CookieSecret == "" {
		cfg.Security.CookieSecret = generateSecretKey()
	}
}

func (cfg *Config) validate() error {
	if cfg.Server.Address == "" {
		return errors.New("server address is empty")
	}
	if cfg.Server.BaseURL == "" {
		return errors.New("base URL is empty")
	}
	parsedURL, err := url.Parse(cfg.Server.BaseURL)
	if err != nil {
		return errors.New("base URL is invalid")
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return errors.New("base URL must contain scheme and host")
	}
	return nil
}

func generateSecretKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random secret: %v", err))
	}
	return hex.EncodeToString(b)
}
