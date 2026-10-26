package database

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL for PostgreSQL tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("portfolio_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close(); _, _ = admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); admin.Close() })
	return pool
}

func TestMigrationUpgradeAndConcurrentStartup(t *testing.T) {
	pool := testDatabase(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	old := t.TempDir()
	for _, file := range []string{"001_create_projects.sql", "002_create_export_jobs.sql", "003_create_model_assets.sql", "004_add_model_asset_metadata.sql", "005_add_users_and_project_owners.sql"} {
		raw, err := os.ReadFile(filepath.Join("../../migrations", file))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(old, file), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := RunMigrations(ctx, pool, old, logger); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO model_assets(id,label,slot,attachment_point,file_name,content_type,format,size_bytes,storage_path,assembly_points)
 VALUES ('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','Old','gear','right_hand','old.obj','text/plain','obj',1,'/old','null')`)
	if err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { errs <- RunMigrations(ctx, pool, "../../migrations", logger) }()
	}
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&count); err != nil || count != 6 {
		t.Fatalf("migrations=%d err=%v", count, err)
	}
	if err := RunMigrations(ctx, pool, t.TempDir(), logger); err == nil {
		t.Fatal("missing migrations accepted")
	}
}

func TestConcurrentAssetAndAnchorRemovalCannotBreakProjects(t *testing.T) {
	for _, operation := range []string{"delete asset", "remove anchor", "delete before save"} {
		t.Run(operation, func(t *testing.T) {
			pool := testDatabase(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := RunMigrations(ctx, pool, "../../migrations", slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
				t.Fatal(err)
			}
			const asset = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
			_, err := pool.Exec(ctx, `INSERT INTO model_assets(id,label,slot,attachment_point,file_name,content_type,format,size_bytes,storage_path,assembly_points)
 VALUES ($1,'Test','gear','right_hand','test.obj','text/plain','obj',1,'/test','[{"id":"grip"}]')`, asset)
			if err != nil {
				t.Fatal(err)
			}
			config, _ := json.Marshal(map[string]any{"assembly": map[string]any{"placements": []any{map[string]string{"id": "p1", "asset_id": asset, "asset_point_id": "grip"}}}})
			first, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer first.Rollback(ctx)
			remove := `DELETE FROM model_assets WHERE id=$1`
			if operation == "remove anchor" {
				remove = `UPDATE model_assets SET assembly_points='[]'::jsonb WHERE id=$1`
			}
			if operation == "delete before save" {
				_, err = first.Exec(ctx, remove, asset)
			} else {
				_, err = first.Exec(ctx, `INSERT INTO projects(name,config) VALUES ('Saved',$1::jsonb)`, string(config))
			}
			if err != nil {
				t.Fatal(err)
			}
			conn, err := pool.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			pid := conn.Conn().PgConn().PID()
			finished := make(chan error, 1)
			go func() {
				if operation == "delete before save" {
					_, err := conn.Exec(ctx, `INSERT INTO projects(name,config) VALUES ('Concurrent',$1::jsonb)`, string(config))
					finished <- err
				} else {
					_, err := conn.Exec(ctx, remove, asset)
					finished <- err
				}
			}()
			// Observe the actual foreign-key lock, rather than depending on timing.
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				var waiting bool
				if err := pool.QueryRow(ctx, `SELECT COALESCE(wait_event_type='Lock',false) FROM pg_stat_activity WHERE pid=$1`, pid).Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting {
					break
				}
				select {
				case err := <-finished:
					t.Fatalf("operation did not wait for FK lock: %v", err)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-ticker.C:
				}
			}
			if err := first.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err := <-finished; !IsReferenceConflict(err) {
				t.Fatalf("expected FK conflict, got %v", err)
			}
			var dangling int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM project_asset_references r LEFT JOIN model_assets a ON a.id=r.asset_id WHERE a.id IS NULL`).Scan(&dangling); err != nil || dangling != 0 {
				t.Fatalf("dangling=%d err=%v", dangling, err)
			}
		})
	}
}
