package app

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCalculationConcurrencyLimit(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	done := make(chan struct{}, 2)
	h := calculationLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { entered <- struct{}{}; <-release; w.WriteHeader(204) }))
	for i := 0; i < 2; i++ {
		go func() {
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/exports/stl", nil))
			done <- struct{}{}
		}()
	}
	<-entered
	<-entered
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/exports/stl", nil))
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatalf("status=%d", w.Code)
	}
	close(release)
	<-done
	<-done
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/exports/stl", nil))
	if w.Code != 204 {
		t.Fatal("permit not released")
	}
}

func TestProxyHeadersAreTrustedOnlyFromConfiguredPeer(t *testing.T) {
	h := TrustedProxy([]string{"172.30.24.10/32"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Client", r.RemoteAddr) }))
	for _, tc := range []struct{ peer, want string }{{"172.30.24.10:1234", "203.0.113.5:0"}, {"192.0.2.1:1234", "192.0.2.1:1234"}} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("X-Real-IP", "203.0.113.5")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Header().Get("Client") != tc.want {
			t.Fatal(w.Header())
		}
	}
}

func TestRequestLoggingRecoversBeforeHeaders(t *testing.T) {
	h := RequestLogging(slog.New(slog.NewTextHandler(io.Discard, nil)), http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("test") }))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 500 || len(w.Header().Get("X-Request-ID")) != 32 {
		t.Fatalf("status=%d headers=%v", w.Code, w.Header())
	}
}
