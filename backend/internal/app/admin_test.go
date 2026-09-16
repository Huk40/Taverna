package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"taverna/backend/internal/catalog"
)

const testAdminToken = "test-only-admin-key-at-least-32-characters"

func TestAssetWriteAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name, token, header, method string
		want                        int
	}{
		{"public read", "", "", "GET", 204},
		{"public head", "", "", "HEAD", 204},
		{"unconfigured upload", "", "", "POST", 503},
		{"weak configuration", "short", "Bearer short", "POST", 503},
		{"anonymous upload", testAdminToken, "", "POST", 401},
		{"anonymous update", testAdminToken, "", "PUT", 401},
		{"wrong deletion key", testAdminToken, "Bearer wrong", "DELETE", 401},
		{"wrong scheme", testAdminToken, "Basic " + testAdminToken, "POST", 401},
		{"valid upload", testAdminToken, "Bearer " + testAdminToken, "POST", 204},
		{"valid update", testAdminToken, "bearer " + testAdminToken, "PUT", 204},
		{"valid deletion", testAdminToken, "Bearer " + testAdminToken, "DELETE", 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			access := AdminAccess{Token: tc.token}
			called := false
			handler := access.protectAssetWrites(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(204)
			})
			req := httptest.NewRequest(tc.method, "/api/model-assets", nil)
			req.Header.Set("Authorization", tc.header)
			res := httptest.NewRecorder()
			handler(res, req)
			if res.Code != tc.want || called != (tc.want == 204) {
				t.Fatalf("status=%d handler called=%v; want status=%d", res.Code, called, tc.want)
			}
		})
	}
}

func TestRouterProtectsAllAssetMutationRoutes(t *testing.T) {
	service, err := catalog.NewService()
	if err != nil {
		t.Fatal(err)
	}
	// Nil repositories ensure unauthorized requests never reach storage.
	router := NewRouter(nil, catalog.NewHandler(service), nil, nil, nil, nil,
		[]string{"http://localhost:5173"}, AdminAccess{Token: testAdminToken, MaxUploadBytes: 52428800})
	for _, endpoint := range []string{"/api/model-assets", "/api/model-assets/", "/api/model-assets/example", "/api/model-assets/example/file"} {
		for _, method := range []string{"POST", "PUT", "DELETE", "PATCH"} {
			res := httptest.NewRecorder()
			router.ServeHTTP(res, httptest.NewRequest(method, endpoint, nil))
			if res.Code != 401 {
				t.Fatalf("%s %s returned %d", method, endpoint, res.Code)
			}
		}
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest("GET", "/api/catalog", nil))
	if res.Code != 200 {
		t.Fatalf("public catalog: %d", res.Code)
	}
	req := httptest.NewRequest("OPTIONS", "/api/model-assets", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != 204 || !strings.Contains(res.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatal("CORS must allow the admin authorization header")
	}
}

func TestAdminSession(t *testing.T) {
	access := AdminAccess{Token: testAdminToken, MaxUploadBytes: 52428800}
	for _, tc := range []struct {
		method, header string
		want           int
	}{
		{"GET", "", 200}, {"POST", "", 401}, {"POST", "Bearer wrong", 401},
		{"POST", "Bearer " + testAdminToken, 200}, {"DELETE", "", 405},
	} {
		req := httptest.NewRequest(tc.method, "/api/admin/session", nil)
		req.Header.Set("Authorization", tc.header)
		res := httptest.NewRecorder()
		access.session(res, req)
		if res.Code != tc.want || res.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: status=%d, headers=%v", tc.method, res.Code, res.Header())
		}
		if strings.Contains(res.Body.String(), testAdminToken) {
			t.Fatal("response leaked admin key")
		}
	}
}
