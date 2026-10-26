package app

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"taverna/backend/internal/httpx"
)

type responseLog struct {
	http.ResponseWriter
	status, bytes int
}

func (w *responseLog) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *responseLog) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}
func (w *responseLog) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func RequestLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		var id [16]byte
		_, _ = rand.Read(id[:])
		requestID := hex.EncodeToString(id[:])
		w.Header().Set("X-Request-ID", requestID)
		recorded := &responseLog{ResponseWriter: w}
		defer func() {
			if failure := recover(); failure != nil {
				logger.Error("request panic", "request_id", requestID, "panic", failure)
				if recorded.status == 0 {
					httpx.Error(recorded, 500, "internal server error")
				}
			}
			if recorded.status == 0 {
				recorded.status = 200
			}
			// Paths may include user IDs; query strings (including tokens) and bodies are never logged.
			logger.Info("http request", "request_id", requestID, "method", r.Method, "path", r.URL.Path, "status", recorded.status, "bytes", recorded.bytes, "duration", time.Since(start))
		}()
		next.ServeHTTP(recorded, r)
	})
}

func calculationLimit(next http.Handler) http.Handler {
	workers := make(chan struct{}, 2)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calculation := r.Method == http.MethodPost && (r.URL.Path == "/api/model-assets" || r.URL.Path == "/api/exports/stl" || strings.HasSuffix(r.URL.Path, "/exports"))
		if calculation {
			select {
			case workers <- struct{}{}:
				defer func() { <-workers }()
			default:
				w.Header().Set("Retry-After", "2")
				httpx.Error(w, 429, "Сервис занят. Повторите запрос позже.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Only the explicitly configured reverse proxy may supply a client address.
func TrustedProxy(cidrs []string, next http.Handler) http.Handler {
	networks := []*net.IPNet{}
	for _, cidr := range cidrs {
		if _, network, err := net.ParseCIDR(cidr); err == nil {
			networks = append(networks, network)
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		for _, network := range networks {
			if network.Contains(net.ParseIP(host)) {
				if ip := net.ParseIP(r.Header.Get("X-Real-IP")); ip != nil {
					clone := r.Clone(r.Context())
					clone.RemoteAddr = net.JoinHostPort(ip.String(), "0")
					r = clone
				}
				break
			}
		}
		next.ServeHTTP(w, r)
	})
}
