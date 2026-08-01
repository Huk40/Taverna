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

	projectConfig, projectExists, err := r.projectConfig(ctx, projectID)
	if err != nil {
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

	artifact, err := buildFakeArtifact(projectID, target, format, projectConfig)
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

func (r *Repository) ListByProject(ctx context.Context, projectID string) ([]ExportJob, error) {
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
		ORDER BY updated_at DESC, created_at DESC
	`, projectID)
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
	row := r.pool.QueryRow(ctx, `
		SELECT id::text, project_id::text, target, format, status, artifact, error, created_at, updated_at
		FROM export_jobs
		WHERE id = $1
	`, id)

	return scanJob(row)
}

func (r *Repository) projectConfig(ctx context.Context, projectID string) (json.RawMessage, bool, error) {
	var raw json.RawMessage
	if err := r.pool.QueryRow(ctx, `SELECT config FROM projects WHERE id = $1`, projectID).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return raw, true, nil
}

func (r *Repository) projectExists(ctx context.Context, projectID string) (bool, error) {
	_, exists, err := r.projectConfig(ctx, projectID)
	return exists, err
}

func buildFakeArtifact(projectID string, target string, format string, projectConfig json.RawMessage) ([]byte, error) {
	return json.Marshal(map[string]any{
		"kind":              "fake_export",
		"schema_version":    1,
		"source_project_id": projectID,
		"target":            target,
		"format":            format,
		"assembly":          assemblySummary(projectConfig),
		"message":           "Placeholder export metadata. Redis, MinIO, and the mesh worker are intentionally not connected yet.",
		"print_readiness": map[string]any{
			"status": "metadata_ready",
			"checks": []string{
				"project_exists",
				"config_validated_before_save",
				"export_job_persisted",
			},
			"warnings": []string{
				"mesh_worker_not_connected",
				"artifact_file_not_generated",
			},
		},
		"worker_plan": []string{"Redis queue", "mesh worker", "MinIO artifact storage"},
	})
}

func assemblySummary(projectConfig json.RawMessage) map[string]any {
	summary := map[string]any{
		"placement_count": 0,
		"asset_ids":       []string{},
		"status":          "no_uploaded_assets",
	}
	if len(projectConfig) == 0 || !json.Valid(projectConfig) {
		return summary
	}

	var config struct {
		Assembly struct {
			Placements []struct {
				AssetID         string `json:"asset_id"`
				Slot            string `json:"slot"`
				AttachmentPoint string `json:"attachment_point"`
			} `json:"placements"`
		} `json:"assembly"`
	}
	if err := json.Unmarshal(projectConfig, &config); err != nil {
		return summary
	}

	assetIDs := make([]string, 0, len(config.Assembly.Placements))
	points := make([]string, 0, len(config.Assembly.Placements))
	for _, placement := range config.Assembly.Placements {
		if placement.AssetID != "" {
			assetIDs = append(assetIDs, placement.AssetID)
		}
		if placement.Slot != "" && placement.AttachmentPoint != "" {
			points = append(points, placement.Slot+"/"+placement.AttachmentPoint)
		}
	}
	summary["placement_count"] = len(config.Assembly.Placements)
	summary["asset_ids"] = assetIDs
	summary["attachment_points"] = points
	if len(config.Assembly.Placements) > 0 {
		summary["status"] = "assembly_plan_ready"
	}
	return summary
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
