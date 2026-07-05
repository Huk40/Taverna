package app

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"taverna/backend/internal/catalog"
	"taverna/backend/internal/exportjob"
	"taverna/backend/internal/project"
)

func NewRouter(projects *project.Handler, catalogHandler *catalog.Handler, exports *exportjob.Handler, pool *pgxpool.Pool, allowedOrigins []string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "error",
				"error":  "database unavailable",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/catalog", catalogHandler.Catalog)
	mux.HandleFunc("/api/catalog/slots", catalogHandler.Slots)
	mux.HandleFunc("/api/catalog/parts", catalogHandler.Parts)
	mux.HandleFunc("/api/catalog/rules", catalogHandler.Rules)
	mux.HandleFunc("/api/projects", projects.Collection)
	mux.HandleFunc("/api/projects/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/exports") {
			exports.ProjectExports(w, r)
			return
		}
		projects.Item(w, r)
	})
	mux.HandleFunc("/api/exports/", exports.Item)

	return corsMiddleware(allowedOrigins)(mux)
}

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (len(allowedOrigins) == 0 || slices.Contains(allowedOrigins, origin)) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}
