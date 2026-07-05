package exportjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("export job not found")
var ErrProjectNotFound = errors.New("project not found")
var ErrInvalidRequest = errors.New("invalid export request")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, projectID string, request CreateRequest) (ExportJob, error) {
	target := normalizeTarget(request.Target)
	format := normalizeFormat(request.Format, target)
	if !allowedTarget(target) {
		return ExportJob{}, fmt.Errorf("%w: unknown target %q", ErrInvalidRequest, target)
	}
	if !allowedFormat(format) {
		return ExportJob{}, fmt.Errorf("%w: unknown format %q", ErrInvalidRequest, format)
	}

	var projectExists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM projects WHERE id = $1)`, projectID).Scan(&projectExists); err != nil {
		return ExportJob{}, fmt.Errorf("check project: %w", err)
	}
	if !projectExists {
		return ExportJob{}, ErrProjectNotFound
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO export_jobs (project_id, target, format, status, artifact)
		VALUES ($1, $2, $3, $4, '{}'::jsonb)
		RETURNING id::text, project_id::text, target, format, status, artifact, error, created_at, updated_at
	`, projectID, target, format, StatusQueued)

	job, err := scanJob(row)
	if err != nil {
		return ExportJob{}, err
	}

	artifact, err := json.Marshal(map[string]any{
		"kind":        "fake_export",
		"target":      target,
		"format":      format,
		"message":     "Placeholder export metadata. Redis, MinIO, and the mesh worker are intentionally not connected yet.",
		"worker_plan": []string{"Redis queue", "mesh worker", "MinIO artifact storage"},
	})
	if err != nil {
		return ExportJob{}, fmt.Errorf("create artifact metadata: %w", err)
	}

	row = r.pool.QueryRow(ctx, `
		UPDATE export_jobs
		SET status = $2,
			artifact = $3::jsonb,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id::text, project_id::text, target, format, status, artifact, error, created_at, updated_at
	`, job.ID, StatusCompleted, string(artifact))

	return scanJob(row)
}

func (r *Repository) Get(ctx context.Context, id string) (ExportJob, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id::text, project_id::text, target, format, status, artifact, error, created_at, updated_at
		FROM export_jobs
		WHERE id = $1
	`, id)

	return scanJob(row)
}

type jobScanner interface {
	Scan(dest ...any) error
}

func scanJob(scanner jobScanner) (ExportJob, error) {
	var job ExportJob
	if err := scanner.Scan(&job.ID, &job.ProjectID, &job.Target, &job.Format, &job.Status, &job.Artifact, &job.Error, &job.CreatedAt, &job.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExportJob{}, ErrNotFound
		}
		return ExportJob{}, fmt.Errorf("scan export job: %w", err)
	}
	return job, nil
}

func normalizeTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return "stl_print"
	}
	return target
}

func normalizeFormat(format string, target string) string {
	format = strings.TrimSpace(format)
	if format != "" {
		return format
	}
	if target == "glb_preview" {
		return "glb"
	}
	return "stl"
}

func allowedTarget(target string) bool {
	switch target {
	case "stl_print", "glb_preview", "physical_print":
		return true
	default:
		return false
	}
}

func allowedFormat(format string) bool {
	switch format {
	case "stl", "glb", "3mf":
		return true
	default:
		return false
	}
}
