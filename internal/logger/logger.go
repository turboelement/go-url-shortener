// Package logger provides context-aware logging using zap.
package logger

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/go-chi/chi/v5/middleware"
)

type loggerKey struct{}

// WithLogger stores a zap logger in the context.
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// FromContext retrieves a zap logger from the context, or returns a no-op logger.
func FromContext(ctx context.Context) *zap.Logger {
	if lg, ok := ctx.Value(loggerKey{}).(*zap.Logger); ok && lg != nil {
		return lg
	}
	return zap.NewNop()
}

// Logger is HTTP middleware that logs each request with method, status, and duration.
func Logger(zaplogger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			reqLogger := zaplogger.With(
				zap.String("method", r.Method),
				zap.String("uri", r.RequestURI),
				zap.String("remote_addr", r.RemoteAddr),
			)

			ctx := WithLogger(r.Context(), reqLogger)
			next.ServeHTTP(ww, r.WithContext(ctx))

			duration := time.Since(start)
			reqLogger.Info("HTTP request handled",
				zap.Int("status", ww.Status()),
				zap.Int("bytes", ww.BytesWritten()),
				zap.Duration("duration", duration),
			)
		})
	}
}
