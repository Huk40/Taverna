package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"taverna/backend/internal/httpx"
)

const sessionCookieName = "taverna_session"

type Handler struct {
	service      *Service
	secureCookie bool
	attempts     loginAttempts
	workers      chan struct{}
}

type loginAttempts struct {
	mu      sync.Mutex
	entries map[string]loginAttempt
}

type loginAttempt struct {
	count   int
	resetAt time.Time
}

func NewHandler(service *Service, secureCookie bool) *Handler {
	return &Handler{service: service, secureCookie: secureCookie, attempts: loginAttempts{entries: make(map[string]loginAttempt)}, workers: make(chan struct{}, 4)}
}

func (h *Handler) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Users must always be able to revoke sessions, even after failed logins.
		logout := r.URL.Path == "/api/auth/logout" || r.URL.Path == "/api/auth/logout-all"
		if strings.HasPrefix(r.URL.Path, "/api/auth/") && r.Method == http.MethodPost && !logout {
			if !h.attempts.allow(clientAddress(r)) {
				w.Header().Set("Retry-After", "900")
				writeJSON(w, 429, map[string]string{"error": "Слишком много запросов. Попробуйте позже."})
				return
			}
			select {
			case h.workers <- struct{}{}:
				defer func() { <-h.workers }()
			default:
				writeJSON(w, 429, map[string]string{"error": "Сервис занят. Повторите попытку."})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	user, _ := IdentityFromContext(r.Context())
	if err := h.service.repo.DeleteAllSessions(r.Context(), user.ID); err != nil {
		writeJSON(w, 503, map[string]string{"error": "Не удалось завершить сеансы"})
		return
	}
	h.clearSessionCookie(w)
	w.WriteHeader(204)
}

func (h *Handler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	user, _ := IdentityFromContext(r.Context())
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := h.service.ResendVerification(ctx, user); err != nil {
		writeJSON(w, 503, map[string]string{"error": "Не удалось отправить письмо. Попробуйте позже."})
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var request struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	user, session, deliveryConfigured, err := h.service.Register(ctx, request.Email, request.DisplayName, request.Password)
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyRegistered) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "Этот email уже зарегистрирован"})
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			writeJSON(w, 400, map[string]string{"error": strings.TrimPrefix(err.Error(), "invalid input: ")})
		} else {
			writeJSON(w, 500, map[string]string{"error": "Не удалось создать аккаунт"})
		}
		return
	}
	h.setSessionCookie(w, session)
	writeJSON(w, http.StatusCreated, map[string]any{"user": user, "verification_sent": deliveryConfigured, "email_delivery_configured": h.service.EmailDeliveryConfigured()})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	user, session, err := h.service.Login(ctx, request.Email, request.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrUserNotFound) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Неверный email или пароль"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось выполнить вход"})
		return
	}
	h.setSessionCookie(w, session)
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "email_delivery_configured": h.service.EmailDeliveryConfigured()})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		err := h.service.Logout(ctx, cookie.Value)
		cancel()
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "Не удалось завершить сеанс. Повторите попытку."})
			return
		}
	}
	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	user, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Требуется вход"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "email_delivery_configured": h.service.EmailDeliveryConfigured()})
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var request struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := h.service.StartPasswordReset(ctx, request.Email); err != nil {
		if errors.Is(err, ErrEmailDeliveryUnavailable) {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Восстановление пароля пока не настроено"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось отправить письмо"})
		return
	}
	// The response does not reveal whether the address has an account.
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var request struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err := h.service.VerifyEmail(ctx, request.Token)
	if err != nil {
		if errors.Is(err, ErrInvalidEmailToken) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Ссылка подтверждения недействительна или устарела"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось подтвердить email"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var request struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	err := h.service.ResetPassword(ctx, request.Token, request.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidEmailToken) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Ссылка восстановления недействительна или устарела"})
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			writeJSON(w, 400, map[string]string{"error": strings.TrimPrefix(err.Error(), "invalid input: ")})
		} else {
			writeJSON(w, 500, map[string]string{"error": "Не удалось изменить пароль"})
		}
		return
	}
	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Optional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok, err := h.userFromRequest(w, r)
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "Не удалось проверить сессию"})
			return
		}
		if ok {
			r = r.WithContext(WithIdentity(r.Context(), user))
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		user, ok, err := h.userFromRequest(w, r)
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "Не удалось проверить сессию"})
			return
		}
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Войдите, чтобы работать с сохранёнными моделями"})
			return
		}
		next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), user)))
	})
}

func (h *Handler) RequireAdmin(next http.Handler) http.Handler {
	return h.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsAdmin(r.Context()) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Требуется роль администратора"})
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func (h *Handler) ProtectAssetWrites(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		h.RequireAdmin(next).ServeHTTP(w, r)
	})
}

func (h *Handler) userFromRequest(w http.ResponseWriter, r *http.Request) (User, bool, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return User{}, false, nil
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	user, err := h.service.UserForSession(ctx, cookie.Value)
	if err != nil {
		if !errors.Is(err, ErrUserNotFound) {
			return User{}, false, err
		}
		h.clearSessionCookie(w)
		return User{}, false, nil
	}
	return user, true, nil
}

func (h *Handler) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/api",
		MaxAge:   int(h.service.SessionTTL().Seconds()),
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/api", MaxAge: -1, HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteLaxMode})
}

func (attempts *loginAttempts) allow(key string) bool {
	attempts.mu.Lock()
	defer attempts.mu.Unlock()
	now := time.Now()
	for k, v := range attempts.entries {
		if v.resetAt.Before(now) {
			delete(attempts.entries, k)
		}
	}
	entry := attempts.entries[key]
	if entry.count >= 30 || len(attempts.entries) >= 10000 {
		return false
	}
	if entry.resetAt.Before(now) {
		entry = loginAttempt{resetAt: now.Add(15 * time.Minute)}
	}
	entry.count++
	attempts.entries[key] = entry
	return true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	return httpx.Decode(w, r, destination, 8<<10, false)
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func clientAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
