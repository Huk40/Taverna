package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("project not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, payload ProjectPayload) (Project, error) {
	payload = normalizePayload(payload)

	row := r.pool.QueryRow(ctx, `
		INSERT INTO projects (name, config)
		VALUES ($1, $2::jsonb)
		RETURNING id::text, name, config, created_at, updated_at
	`, payload.Name, string(payload.Config))

	return scanProject(row)
}

func (r *Repository) List(ctx context.Context) ([]Project, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, name, config, created_at, updated_at
		FROM projects
		ORDER BY updated_at DESC, created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	defer rows.Close()

	projects := make([]Project, 0)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read projects: %w", err)
	}

	return projects, nil
}

func (r *Repository) Get(ctx context.Context, id string) (Project, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id::text, name, config, created_at, updated_at
		FROM projects
		WHERE id = $1
	`, id)

	return scanProject(row)
}

func (r *Repository) Update(ctx context.Context, id string, payload ProjectPayload) (Project, error) {
	payload = normalizePayload(payload)

	row := r.pool.QueryRow(ctx, `
		UPDATE projects
		SET name = $2,
			config = $3::jsonb,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id::text, name, config, created_at, updated_at
	`, id, payload.Name, string(payload.Config))

	return scanProject(row)
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type projectScanner interface {
	Scan(dest ...any) error
}

func scanProject(scanner projectScanner) (Project, error) {
	var p Project
	if err := scanner.Scan(&p.ID, &p.Name, &p.Config, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Project{}, ErrNotFound
		}
		return Project{}, fmt.Errorf("scan project: %w", err)
	}
	return p, nil
}

func normalizePayload(payload ProjectPayload) ProjectPayload {
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		payload.Name = "Untitled hero"
	}

	if len(payload.Config) > 0 && !json.Valid(payload.Config) {
		payload.Config = []byte(`{}`)
	}

	return payload
}
