package app

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"taverna/backend/internal/assembly"
	"taverna/backend/internal/auth"
	"taverna/backend/internal/catalog"
	"taverna/backend/internal/exportjob"
	"taverna/backend/internal/modelasset"
	"taverna/backend/internal/project"
)

func NewRouter(projects *project.Handler, catalogHandler *catalog.Handler, exports *exportjob.Handler, modelAssets *modelasset.Handler, assemblyHandler *assembly.Handler, authHandler *auth.Handler, pool *pgxpool.Pool, allowedOrigins []string) http.Handler {
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
	mux.HandleFunc("/api/assembly/preview", assemblyHandler.PreviewAssembly)
	mux.HandleFunc("/api/exports/stl", exports.PrintSTL)
	mux.HandleFunc("/api/auth/register", authHandler.Register)
	mux.HandleFunc("/api/auth/login", authHandler.Login)
	mux.HandleFunc("/api/auth/logout", authHandler.Logout)
	mux.Handle("/api/auth/logout-all", authHandler.RequireUser(http.HandlerFunc(authHandler.LogoutAll)))
	mux.Handle("/api/auth/resend-verification", authHandler.RequireUser(http.HandlerFunc(authHandler.ResendVerification)))
	mux.Handle("/api/auth/me", authHandler.Optional(http.HandlerFunc(authHandler.Me)))
	mux.HandleFunc("/api/auth/forgot-password", authHandler.ForgotPassword)
	mux.HandleFunc("/api/auth/verify-email", authHandler.VerifyEmail)
	mux.HandleFunc("/api/auth/reset-password", authHandler.ResetPassword)
	mux.Handle("/api/model-assets", authHandler.ProtectAssetWrites(modelAssets.Collection))
	mux.HandleFunc("/api/model-assets/assembly-points", modelAssets.AssemblyPoints)
	mux.Handle("/api/model-assets/", authHandler.ProtectAssetWrites(modelAssets.Item))
	mux.Handle("/api/projects", authHandler.RequireUser(http.HandlerFunc(projects.Collection)))
	mux.Handle("/api/projects/", authHandler.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/exports") {
			exports.ProjectExports(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/assembly") {
			assemblyHandler.ProjectAssembly(w, r)
			return
		}
		projects.Item(w, r)
	})))
	mux.Handle("/api/exports/", authHandler.RequireUser(http.HandlerFunc(exports.Item)))

	return corsMiddleware(allowedOrigins)(calculationLimit(authHandler.Limit(mux)))
}

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Add("Vary", "Origin")
			if origin != "" && !slices.Contains(allowedOrigins, origin) {
				writeJSON(w, 403, map[string]string{"error": "Источник запроса не разрешён"})
				return
			}
			if origin != "" && slices.Contains(allowedOrigins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Taverna-Request")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Expose-Headers", "X-Next-Offset, X-Request-ID")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// Custom headers force a CORS preflight and block form-based CSRF,
			// including login CSRF and multipart uploads from sibling subdomains.
			safe := r.Method == http.MethodGet || r.Method == http.MethodHead
			publicCalculation := r.URL.Path == "/api/assembly/preview" || r.URL.Path == "/api/exports/stl"
			if !safe && !publicCalculation && r.Header.Get("X-Taverna-Request") != "1" {
				writeJSON(w, 403, map[string]string{"error": "Отсутствует заголовок защиты запроса"})
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
