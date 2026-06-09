package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-url-shortener/internal/audit"
	"go-url-shortener/internal/config"
	"go-url-shortener/internal/profiler"
	"go-url-shortener/internal/repository"
	"go-url-shortener/internal/server"
	"go-url-shortener/internal/service"

	"go.uber.org/zap"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func formatBuildInfo(label, value string) string {
	if value == "" {
		value = "N/A"
	}
	return fmt.Sprintf("%s: %s", label, value)
}

func main() {
	fmt.Println(formatBuildInfo("Build version", buildVersion))
	fmt.Println(formatBuildInfo("Build date", buildDate))
	fmt.Println(formatBuildInfo("Build commit", buildCommit))

	cfg := config.New()

	logger, err := zap.NewProduction() // or zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize zap logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Server configuration loaded",
		zap.String("ServerAddr", cfg.ServerAddr),
		zap.String("BaseURL", cfg.BaseURL),
	)

	if cfg.EnablePprof {
		p := profiler.New()
		p.Start()
		defer p.Close()
		logger.Info("pprof server started", zap.String("address", p.Addr()))
	}

	var repo repository.URLRepositoryInterface

	if cfg.DatabaseDSN != "" {
		logger.Info("Using Database")
		dbRepo, err := repository.NewPostgresRepository(cfg.DatabaseDSN)
		if err != nil {
			logger.Fatal("Error connecting to DB", zap.Error(err))
		}
		defer dbRepo.Close()
		repo = dbRepo
	} else if cfg.FileStoragePath != "" {
		logger.Info("Using file storage",
			zap.String("file_path", cfg.FileStoragePath),
		)
		repo = repository.NewFileURLRepository(cfg.FileStoragePath)
	} else {
		logger.Info("Using in-memory storage")
		repo = repository.NewURLRepository()
	}

	svc := service.NewShortenerService(repo)
	defer svc.Close()

	auditSubject := audit.NewSubject()
	if cfg.AuditFilePath != "" {
		fo, err := audit.NewFileObserver(cfg.AuditFilePath)
		if err != nil {
			logger.Fatal("failed to create file audit observer", zap.Error(err))
		}
		auditSubject.Register(fo)
		logger.Info("Using file audit", zap.String("file_path", cfg.AuditFilePath))
	}
	if cfg.AuditURL != "" {
		auditSubject.Register(audit.NewHTTPObserver(cfg.AuditURL))
		logger.Info("Using HTTP audit", zap.String("url", cfg.AuditURL))
	}
	defer auditSubject.Close()

	router := server.NewRouter(server.RouterDeps{
		BaseURL:      cfg.BaseURL,
		CookieSecret: cfg.CookieSecret,
		Logger:       logger,
		Repo:         repo,
		Svc:          svc,
		AuditSubject: auditSubject,
	})

	srv := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	logger.Info("Server running at",
		zap.String("address", cfg.BaseURL),
		zap.Bool("https", cfg.EnableHTTPS),
	)

	if cfg.EnableHTTPS {
		tlsConfig, err := newTLSConfig(cfg.ServerAddr)
		if err != nil {
			logger.Fatal("failed to initialize TLS config", zap.Error(err))
		}

		listener, err := tls.Listen("tcp", cfg.ServerAddr, tlsConfig)
		if err != nil {
			logger.Fatal("failed to start TLS listener", zap.Error(err))
		}

		logger.Info("HTTPS server started with self-signed certificate")

		go func() {
			if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
				logger.Error("server error", zap.Error(err))
			}
		}()
	} else {
		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("server starting failed", zap.Error(err))
			}
		}()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logger.Info("Server shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown failed", zap.Error(err))
	}

	logger.Info("Server stopped")
}

// newTLSConfig returns a tls.Config with a self-signed certificate.
func newTLSConfig(addr string) (*tls.Config, error) {
	cert, err := newSelfSignedCertificate(addr)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// newSelfSignedCertificate generates a self-signed TLS certificate for the given address.
func newSelfSignedCertificate(addr string) (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: host,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
	}

	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{host}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	return tls.X509KeyPair(certPEM, keyPEM)
}
