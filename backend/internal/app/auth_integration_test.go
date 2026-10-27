package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"taverna/backend/internal/assembly"
	"taverna/backend/internal/auth"
	"taverna/backend/internal/catalog"
	"taverna/backend/internal/database"
	"taverna/backend/internal/exportjob"
	"taverna/backend/internal/modelasset"
	"taverna/backend/internal/project"
)

type testMail struct {
	mu            sync.Mutex
	verify, reset map[string]string
}

func (m *testMail) Configured() bool { return true }
func (m *testMail) SendVerification(_ context.Context, email, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verify[email] = token
	return nil
}
func (m *testMail) SendPasswordReset(_ context.Context, email, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reset[email] = token
	return nil
}
func (m *testMail) token(email, kind string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if kind == "verify" {
		return m.verify[email]
	}
	return m.reset[email]
}

// Uses a fresh schema, never the application's projects or users.
func TestAccountsAndPrivateProjects(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("auth_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE")
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err = database.RunMigrations(ctx, pool, "../../migrations", logger); err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.NewService()
	if err != nil {
		t.Fatal(err)
	}
	projects := project.NewRepository(pool)
	assets := modelasset.NewRepository(pool)
	service := assembly.NewService(projects, assets, cat)
	out := t.TempDir()
	exports := exportjob.NewRepository(pool)
	mail := &testMail{verify: map[string]string{}, reset: map[string]string{}}
	authHandler := auth.NewHandler(auth.NewService(auth.NewRepository(pool), auth.Config{Mailer: mail}), false)
	router := NewRouter(project.NewHandler(projects, cat, assets, logger), catalog.NewHandler(cat),
		exportjob.NewHandler(exports, exportjob.NewService(exports, exportjob.NewArtifactBuilder(out)), service, out, logger), modelasset.NewHandler(assets, cat, t.TempDir(), 1024*1024, logger),
		assembly.NewHandler(service, logger), authHandler, pool, []string{"http://localhost:5173"})
	server := httptest.NewServer(router)
	defer server.Close()
	newClient := func() *http.Client { jar, _ := cookiejar.New(nil); return &http.Client{Jar: jar} }
	alice, bob, guest := newClient(), newClient(), newClient()
	request := func(client *http.Client, method, path string, payload any, want int) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(payload)
		req, _ := http.NewRequest(method, server.URL+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Taverna-Request", "1")
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("%s %s got %d want %d: %s", method, path, res.StatusCode, want, data)
		}
		if strings.Contains(string(data), "password_hash") {
			t.Fatal("password hash exposed")
		}
		var object map[string]any
		_ = json.Unmarshal(data, &object)
		return object
	}
	emailA, emailB := "alice@example.test", "bob@example.test"
	password := "strong-test-password"
	credentials := func(email string) map[string]string { return map[string]string{"email": email, "password": password} }
	a := request(alice, "POST", "/api/auth/register", credentials(emailA), 201)
	userA := a["user"].(map[string]any)["id"].(string)
	if a["user"].(map[string]any)["role"] != "user" {
		t.Fatal("registration granted privileges")
	}
	request(bob, "POST", "/api/auth/register", credentials(emailB), 201)
	request(guest, "POST", "/api/auth/register", credentials("ALICE@example.test"), 409)
	request(alice, "GET", "/api/auth/me", nil, 200)
	request(guest, "GET", "/api/projects", nil, 401)
	request(alice, "POST", "/api/model-assets", nil, 403)
	request(guest, "POST", "/api/model-assets", nil, 401)
	var storedHash string
	if err = pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE id=$1", userA).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(storedHash, "argon2id$") || strings.Contains(storedHash, password) {
		t.Fatal("invalid stored password")
	}
	c := request(guest, "GET", "/api/catalog", nil, 200)["defaults"]
	assetID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	source := filepath.Join(t.TempDir(), "triangle.obj")
	if err := os.WriteFile(source, []byte("v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := assets.Create(ctx, modelasset.CreateRequest{ID: assetID, Label: "Triangle", Slot: "gear", AttachmentPoint: "right_hand", FileName: "triangle.obj", ContentType: "text/plain", Format: "obj", SizeBytes: 32, StoragePath: source}); err != nil {
		t.Fatal(err)
	}
	c.(map[string]any)["assembly"] = map[string]any{"placements": []any{map[string]any{"id": "one", "asset_id": assetID, "slot": "gear", "attachment_point": "right_hand", "transform": map[string]any{"scale": 1, "position": []int{0, 0, 0}, "rotation": []int{0, 0, 0}}}}}
	p := request(alice, "POST", "/api/projects", map[string]any{"name": "Alice model", "config": c}, 201)
	id := p["id"].(string)
	for client, expected := range map[*http.Client]int{alice: 1, bob: 0} {
		res, err := client.Get(server.URL + "/api/projects")
		if err != nil {
			t.Fatal(err)
		}
		var listed []project.Project
		err = json.NewDecoder(res.Body).Decode(&listed)
		res.Body.Close()
		if err != nil || res.StatusCode != 200 || len(listed) != expected {
			t.Fatalf("project list leaked or omitted models: %v %v", listed, err)
		}
	}
	request(alice, "GET", "/api/projects/"+id, nil, 200)
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		request(bob, method, "/api/projects/"+id, map[string]any{"name": "stolen", "config": c}, 404)
	}
	request(bob, "GET", "/api/projects/"+id+"/assembly", nil, 404)
	request(bob, "GET", "/api/projects/"+id+"/exports", nil, 404)
	request(bob, "POST", "/api/projects/"+id+"/exports", map[string]string{"format": "obj"}, 404)
	job := request(alice, "POST", "/api/projects/"+id+"/exports", map[string]string{"format": "obj"}, 201)
	jobID := job["id"].(string)
	request(bob, "GET", "/api/exports/"+jobID, nil, 404)
	request(bob, "GET", "/api/exports/"+jobID+"/artifact", nil, 404)
	request(alice, "GET", "/api/exports/"+jobID, nil, 200)
	request(alice, "GET", "/api/exports/"+jobID+"/artifact", nil, 200)
	request(alice, "PUT", "/api/projects/"+id, map[string]any{"name": "Updated model", "config": c}, 200)
	for _, path := range []string{"/api/projects/invalid", "/api/model-assets/invalid", "/api/exports/invalid", "/api/projects/invalid/assembly", "/api/projects/invalid/exports"} {
		request(alice, "GET", path, nil, 404)
	}
	request(alice, "GET", "/api/projects?limit=0", nil, 400)
	request(alice, "POST", "/api/projects", map[string]any{"name": strings.Repeat("я", 121), "config": c}, 400)
	request(alice, "POST", "/api/projects/"+id+"/exports", map[string]string{"format": "glb"}, 400)
	res, err := alice.Get(server.URL + "/api/projects?limit=1")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.Header.Get("X-Next-Offset") != "1" {
		t.Fatal("pagination cursor missing")
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	request(alice, "POST", "/api/projects/"+id+"/exports", map[string]string{"format": "stl"}, 500)
	var failed int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM export_jobs WHERE project_id=$1 AND status='failed' AND error IS NOT NULL", id).Scan(&failed); err != nil || failed != 1 {
		t.Fatalf("failed exports=%d err=%v", failed, err)
	}
	files, err := os.ReadDir(out)
	if err != nil || len(files) != 1 {
		t.Fatalf("partial file retained: files=%v err=%v", files, err)
	}
	// CSRF is blocked even when the attacker has a same-site cookie.
	for _, origin := range []string{"http://evil.example", "http://localhost:5173"} {
		req, _ := http.NewRequest("POST", server.URL+"/api/auth/logout", nil)
		req.Header.Set("Origin", origin)
		if origin == "http://evil.example" {
			req.Header.Set("X-Taverna-Request", "1")
		}
		res, err := alice.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 403 {
			t.Fatalf("CSRF status %d", res.StatusCode)
		}
	}
	request(alice, "GET", "/api/auth/me", nil, 200)
	token := mail.token(emailA, "verify")
	request(guest, "POST", "/api/auth/verify-email", map[string]string{"token": token}, 204)
	request(guest, "POST", "/api/auth/verify-email", map[string]string{"token": token}, 400)
	request(guest, "GET", "/api/auth/me", nil, 401)
	if request(alice, "GET", "/api/auth/me", nil, 200)["user"].(map[string]any)["email_verified"] != true {
		t.Fatal("email not verified")
	}
	request(guest, "POST", "/api/auth/forgot-password", map[string]string{"email": "absent@example.test"}, 204)
	request(guest, "POST", "/api/auth/forgot-password", map[string]string{"email": emailA}, 204)
	reset := mail.token(emailA, "reset")
	nextPassword := "new-strong-password"
	request(guest, "POST", "/api/auth/reset-password", map[string]string{"token": reset, "password": nextPassword}, 204)
	request(alice, "GET", "/api/auth/me", nil, 401)
	request(guest, "POST", "/api/auth/reset-password", map[string]string{"token": reset, "password": nextPassword}, 400)
	request(alice, "POST", "/api/auth/login", credentials(emailA), 401)
	changed := map[string]string{"email": emailA, "password": nextPassword}
	login := request(alice, "POST", "/api/auth/login", changed, 200)
	if login["user"].(map[string]any)["id"] != userA {
		t.Fatal("reset changed account")
	}
	second := newClient()
	request(second, "POST", "/api/auth/login", changed, 200)
	request(alice, "POST", "/api/auth/logout-all", nil, 204)
	request(second, "GET", "/api/auth/me", nil, 401)
	request(alice, "POST", "/api/auth/login", changed, 200)
	if _, err = pool.Exec(ctx, "UPDATE user_sessions SET expires_at=NOW()-INTERVAL '1 second' WHERE user_id=$1", userA); err != nil {
		t.Fatal(err)
	}
	request(alice, "GET", "/api/auth/me", nil, 401)
	request(bob, "POST", "/api/auth/resend-verification", nil, 204)
	expired := mail.token(emailB, "verify")
	if _, err = pool.Exec(ctx, "UPDATE user_email_tokens SET expires_at=NOW()-INTERVAL '1 second'"); err != nil {
		t.Fatal(err)
	}
	request(guest, "POST", "/api/auth/verify-email", map[string]string{"token": expired}, 400)
	request(bob, "POST", "/api/auth/logout", nil, 204)
	request(bob, "GET", "/api/projects", nil, 401)
	// Server-side promotion is reflected without trusting a role in a cookie.
	if _, err = pool.Exec(ctx, "UPDATE users SET role='admin' WHERE id=$1", userA); err != nil {
		t.Fatal(err)
	}
	request(alice, "POST", "/api/auth/login", changed, 200)
	request(alice, "POST", "/api/model-assets", nil, 400) // passed role check, invalid multipart
	request(alice, "DELETE", "/api/model-assets/"+assetID, nil, 409)
	request(alice, "DELETE", "/api/projects/"+id, nil, 204)
	request(alice, "GET", "/api/projects/"+id, nil, 404)

	// Repository access also fails closed without identity.
	if _, err = projects.Get(ctx, id); !errorsIsNotFound(err) {
		t.Fatal("unscoped project read allowed")
	}
	if _, err = exports.Get(ctx, jobID); err != exportjob.ErrNotFound {
		t.Fatal("unscoped export read allowed")
	}
}
func errorsIsNotFound(err error) bool { return err == project.ErrNotFound }
