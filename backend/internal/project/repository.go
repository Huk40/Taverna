package project

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

var ErrNotFound = errors.New("project not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, payload ProjectPayload) (Project, error) {
	user, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return Project{}, ErrNotFound
	}
	return r.CreateForOwner(ctx, user.ID, payload)
}

func (r *Repository) CreateForOwner(ctx context.Context, ownerID string, payload ProjectPayload) (Project, error) {
	payload = normalizePayload(payload)

	row := r.pool.QueryRow(ctx, `
		INSERT INTO projects (owner_id, name, config)
		VALUES ($1, $2, $3::jsonb)
		RETURNING id::text, name, config, created_at, updated_at
	`, ownerID, payload.Name, string(payload.Config))

	return scanProject(row)
}

func (r *Repository) List(ctx context.Context) ([]Project, error) {
	user, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, ErrNotFound
	}
	return r.ListForOwner(ctx, user.ID)
}

func (r *Repository) ListForOwner(ctx context.Context, ownerID string, pages ...httpx.Page) ([]Project, error) {
	page := httpx.Page{Limit: 50}
	if len(pages) > 0 {
		page = pages[0]
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, name, config, created_at, updated_at
		FROM projects
		WHERE owner_id = $1
		ORDER BY updated_at DESC, id DESC LIMIT $2 OFFSET $3
	`, ownerID, page.Limit, page.Offset)
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
	user, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return Project{}, ErrNotFound
	}
	return r.GetForOwner(ctx, id, user.ID)
}

func (r *Repository) GetForOwner(ctx context.Context, id, ownerID string) (Project, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id::text, name, config, created_at, updated_at
		FROM projects
		WHERE id = $1 AND owner_id = $2
	`, id, ownerID)
	return scanProject(row)
}

func (r *Repository) Update(ctx context.Context, id string, payload ProjectPayload) (Project, error) {
	user, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return Project{}, ErrNotFound
	}
	return r.UpdateForOwner(ctx, id, user.ID, payload)
}

func (r *Repository) UpdateForOwner(ctx context.Context, id, ownerID string, payload ProjectPayload) (Project, error) {
	payload = normalizePayload(payload)

	row := r.pool.QueryRow(ctx, `
		UPDATE projects
		SET name = $2,
			config = $3::jsonb,
			updated_at = NOW()
		WHERE id = $1 AND owner_id = $4
		RETURNING id::text, name, config, created_at, updated_at
	`, id, payload.Name, string(payload.Config), ownerID)

	return scanProject(row)
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	user, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return ErrNotFound
	}
	return r.DeleteForOwner(ctx, id, user.ID)
}

func (r *Repository) DeleteForOwner(ctx context.Context, id, ownerID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1 AND owner_id = $2`, id, ownerID)
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
