package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"taverna/backend/internal/auth"
	"taverna/backend/internal/catalog"
)

func TestRouterProtectsAssetMutationRoutes(t *testing.T) {
	service, err := catalog.NewService()
	if err != nil {
		t.Fatal(err)
	}
	// Nil storage handlers are safe here: unauthenticated writes must be rejected
	// before a request can reach them.
	router := NewRouter(nil, catalog.NewHandler(service), nil, nil, nil, auth.NewHandler(nil, false), nil,
		[]string{"http://localhost:5173"})
	for _, endpoint := range []string{"/api/model-assets", "/api/model-assets/", "/api/model-assets/example", "/api/model-assets/example/file"} {
		for _, method := range []string{"POST", "PUT", "DELETE", "PATCH"} {
			res := httptest.NewRecorder()
			request := httptest.NewRequest(method, endpoint, nil)
			request.Header.Set("X-Taverna-Request", "1")
			router.ServeHTTP(res, request)
			if res.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s returned %d", method, endpoint, res.Code)
			}
		}
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/catalog", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("public catalog: %d", res.Code)
	}
	req := httptest.NewRequest(http.MethodOptions, "/api/model-assets", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent || !strings.Contains(res.Header().Get("Access-Control-Allow-Headers"), "Content-Type") || res.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("invalid CORS response: %d %v", res.Code, res.Header())
	}
}
