// Package middleware provides HTTP middleware for the URL shortener.
package middleware

import (
	"compress/gzip"
	"net"
	"net/http"
	"strings"
)

// TrustedSubnetMiddleware checks whether the IP from the X-Real-IP header
// belongs to the trusted subnet (CIDR). If not, returns 403 Forbidden.
// Set trustedSubnet to empty string to always deny access.
func TrustedSubnetMiddleware(trustedSubnet string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if trustedSubnet == "" {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			_, cidrNet, err := net.ParseCIDR(trustedSubnet)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			realIP := r.Header.Get("X-Real-IP")
			if realIP == "" {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			ip := net.ParseIP(realIP)
			if ip == nil {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			if !cidrNet.Contains(ip) {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Decompress is middleware that decompresses gzip-encoded request bodies.
func Decompress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gr, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "Invalid gzip content", http.StatusBadRequest)
			return
		}
		defer gr.Close()

		r.Body = gr
		r.Header.Del("Content-Length")

		next.ServeHTTP(w, r)
	})
}
