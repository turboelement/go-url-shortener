package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"
)

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
