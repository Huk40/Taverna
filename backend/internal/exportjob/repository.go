package exportjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"taverna/backend/internal/auth"
	"taverna/backend/internal/httpx"
)

var ErrNotFound = errors.New("export job not found")
var ErrProjectNotFound = errors.New("project not found")
var ErrInvalidRequest = errors.New("invalid export request")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Insert(ctx context.Context, projectID, target, format string) (ExportJob, error) {
	exists, err := r.projectExists(ctx, projectID)
	if err != nil {
		return ExportJob{}, err
	}
	if !exists {
		return ExportJob{}, ErrProjectNotFound
	}
	return scanJob(r.pool.QueryRow(ctx, `INSERT INTO export_jobs(project_id,target,format,status)
 VALUES ($1,$2,$3,'processing') RETURNING id::text, project_id::text, target, format, status, artifact, error, created_at, updated_at`, projectID, target, format))
}
func (r *Repository) Complete(ctx context.Context, id string, artifact json.RawMessage) (ExportJob, error) {
	return scanJob(r.pool.QueryRow(ctx, `UPDATE export_jobs SET status='completed', artifact=$2::jsonb, error=NULL, updated_at=NOW()
 WHERE id=$1 RETURNING id::text, project_id::text, target, format, status, artifact, error, created_at, updated_at`, id, string(artifact)))
}
func (r *Repository) Fail(ctx context.Context, id, reason string) error {
	_, err := r.pool.Exec(ctx, `UPDATE export_jobs SET status='failed', error=$2, updated_at=NOW() WHERE id=$1`, id, reason)
	return err
}

func (r *Repository) ListByProject(ctx context.Context, projectID string, pages ...httpx.Page) ([]ExportJob, error) {
	page := httpx.Page{Limit: 50}
	if len(pages) > 0 {
		page = pages[0]
	}
	projectExists, err := r.projectExists(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("check project: %w", err)
	}
	if !projectExists {
		return nil, ErrProjectNotFound
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id::text, project_id::text, target, format, status, artifact, error, created_at, updated_at
		FROM export_jobs
		WHERE project_id = $1
		ORDER BY updated_at DESC, id DESC LIMIT $2 OFFSET $3
	`, projectID, page.Limit, page.Offset)
	if err != nil {
		return nil, fmt.Errorf("list export jobs: %w", err)
	}
	defer rows.Close()

	jobs := make([]ExportJob, 0)
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list export jobs: %w", err)
	}

	return jobs, nil
}

func (r *Repository) Get(ctx context.Context, id string) (ExportJob, error) {
	if user, ok := auth.IdentityFromContext(ctx); ok {
		row := r.pool.QueryRow(ctx, `
			SELECT e.id::text, e.project_id::text, e.target, e.format, e.status, e.artifact, e.error, e.created_at, e.updated_at
			FROM export_jobs e
			JOIN projects p ON p.id = e.project_id
			WHERE e.id = $1 AND p.owner_id = $2
		`, id, user.ID)
		return scanJob(row)
	}
	return ExportJob{}, ErrNotFound
}

func (r *Repository) projectExists(ctx context.Context, projectID string) (bool, error) {
	var exists bool
	user, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return false, nil
	}
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM projects WHERE id=$1 AND owner_id=$2)`, projectID, user.ID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
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
	case "stl", "obj":
		return true
	default:
		return false
	}
}
