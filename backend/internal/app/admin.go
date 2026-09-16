package app

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

// A high-entropy server-side key grants library management access. An unset or
// short key disables writes; public browsing and assembly remain available.
type AdminAccess struct {
	Token          string
	MaxUploadBytes int64
}

func (a AdminAccess) configured() bool { return len(a.Token) >= 32 }

func (a AdminAccess) authorize(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	if !a.configured() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Вход администратора не настроен на сервере"})
		return false
	}
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		provided := sha256.Sum256([]byte(parts[1]))
		expected := sha256.Sum256([]byte(a.Token))
		if subtle.ConstantTimeCompare(provided[:], expected[:]) == 1 {
			return true
		}
	}
	w.Header().Set("WWW-Authenticate", `Bearer realm="asset administration"`)
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Требуется действующий ключ администратора"})
	return false
}

func (a AdminAccess) protectAssetWrites(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if !a.authorize(w, r) {
				return
			}
		}
		next(w, r)
	}
}

func (a AdminAccess) session(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"configured":       a.configured(),
			"max_upload_bytes": a.MaxUploadBytes,
		})
	case http.MethodPost:
		if a.authorize(w, r) {
			writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
		}
	default:
		methodNotAllowed(w)
	}
}
