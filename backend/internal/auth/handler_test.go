package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionCookieAndRateLimit(t *testing.T) {
	h := NewHandler(NewService(nil, Config{}), true)
	w := httptest.NewRecorder()
	h.setSessionCookie(w, "test-token")
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].MaxAge <= 0 {
		t.Fatal("insecure session cookie")
	}
	called := 0
	endpoint := h.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called++; w.WriteHeader(204) }))
	for i := 0; i < 31; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/auth/register", nil)
		endpoint.ServeHTTP(w, req)
		if i == 30 && w.Code != 429 {
			t.Fatal("missing rate limit")
		}
	}
	if called != 30 {
		t.Fatal(called)
	}
	for _, path := range []string{"/api/auth/logout", "/api/auth/logout-all"} {
		w := httptest.NewRecorder()
		endpoint.ServeHTTP(w, httptest.NewRequest("POST", path, nil))
		if w.Code != 204 {
			t.Fatal("rate limit prevented logout")
		}
	}
	h.attempts.entries = map[string]loginAttempt{"expired": {count: 30, resetAt: time.Now().Add(-time.Second)}}
	if !h.attempts.allow("expired") {
		t.Fatal("expired rate limit not released")
	}
}
