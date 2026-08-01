package modelasset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("model asset not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, request CreateRequest) (ModelAsset, error) {
	request = normalizeCreateRequest(request)
	points, err := json.Marshal(request.AssemblyPoints)
	if err != nil {
		return ModelAsset{}, fmt.Errorf("marshal assembly points: %w", err)
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO model_assets (
			id, label, slot, attachment_point, file_name, content_type, format, size_bytes, storage_path, assembly_points
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb)
		RETURNING id::text, label, slot, attachment_point, file_name, content_type, format, size_bytes, storage_path, assembly_points, created_at, updated_at
	`, request.ID, request.Label, request.Slot, request.AttachmentPoint, request.FileName, request.ContentType, request.Format, request.SizeBytes, request.StoragePath, string(points))

	return scanAsset(row)
}

func (r *Repository) List(ctx context.Context, slot string) ([]ModelAsset, error) {
	var (
		rows pgx.Rows
		err  error
	)

	if slot == "" {
		rows, err = r.pool.Query(ctx, `
			SELECT id::text, label, slot, attachment_point, file_name, content_type, format, size_bytes, storage_path, assembly_points, created_at, updated_at
			FROM model_assets
			ORDER BY updated_at DESC, created_at DESC
		`)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT id::text, label, slot, attachment_point, file_name, content_type, format, size_bytes, storage_path, assembly_points, created_at, updated_at
			FROM model_assets
			WHERE slot = $1
			ORDER BY updated_at DESC, created_at DESC
		`, strings.TrimSpace(slot))
	}
	if err != nil {
		return nil, fmt.Errorf("query model assets: %w", err)
	}
	defer rows.Close()

	assets := make([]ModelAsset, 0)
	for rows.Next() {
		asset, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read model assets: %w", err)
	}

	return assets, nil
}

func (r *Repository) Get(ctx context.Context, id string) (ModelAsset, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id::text, label, slot, attachment_point, file_name, content_type, format, size_bytes, storage_path, assembly_points, created_at, updated_at
		FROM model_assets
		WHERE id = $1
	`, strings.TrimSpace(id))

	return scanAsset(row)
}

func (r *Repository) Update(ctx context.Context, id string, request UpdateRequest) (ModelAsset, error) {
	request = normalizeUpdateRequest(request)
	points, err := json.Marshal(request.AssemblyPoints)
	if err != nil {
		return ModelAsset{}, fmt.Errorf("marshal assembly points: %w", err)
	}

	row := r.pool.QueryRow(ctx, `
		UPDATE model_assets
		SET label = $2,
			assembly_points = $3::jsonb,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id::text, label, slot, attachment_point, file_name, content_type, format, size_bytes, storage_path, assembly_points, created_at, updated_at
	`, strings.TrimSpace(id), request.Label, string(points))

	return scanAsset(row)
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM model_assets WHERE id = $1`, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("delete model asset: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) MissingIDs(ctx context.Context, ids []string) ([]string, error) {
	unique := uniqueNonEmpty(ids)
	if len(unique) == 0 {
		return nil, nil
	}

	rows, err := r.pool.Query(ctx, `SELECT id::text FROM model_assets WHERE id::text = ANY($1)`, unique)
	if err != nil {
		return nil, fmt.Errorf("query model asset ids: %w", err)
	}
	defer rows.Close()

	found := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan model asset id: %w", err)
		}
		found[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read model asset ids: %w", err)
	}

	missing := make([]string, 0)
	for _, id := range unique {
		if _, ok := found[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

type assetScanner interface {
	Scan(dest ...any) error
}

func scanAsset(scanner assetScanner) (ModelAsset, error) {
	var (
		asset     ModelAsset
		rawPoints []byte
	)
	if err := scanner.Scan(
		&asset.ID,
		&asset.Label,
		&asset.Slot,
		&asset.AttachmentPoint,
		&asset.FileName,
		&asset.ContentType,
		&asset.Format,
		&asset.SizeBytes,
		&asset.StoragePath,
		&rawPoints,
		&asset.CreatedAt,
		&asset.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ModelAsset{}, ErrNotFound
		}
		return ModelAsset{}, fmt.Errorf("scan model asset: %w", err)
	}

	if len(rawPoints) > 0 {
		if err := json.Unmarshal(rawPoints, &asset.AssemblyPoints); err != nil {
			return ModelAsset{}, fmt.Errorf("decode model asset assembly points: %w", err)
		}
	}
	if asset.AssemblyPoints == nil {
		asset.AssemblyPoints = []AssemblyPoint{}
	}
	asset.URL = "/api/model-assets/" + asset.ID + "/file"

	return asset, nil
}

func normalizeCreateRequest(request CreateRequest) CreateRequest {
	request.Label = strings.TrimSpace(request.Label)
	if request.Label == "" {
		request.Label = request.FileName
	}
	request.Slot = strings.TrimSpace(request.Slot)
	request.AttachmentPoint = strings.TrimSpace(request.AttachmentPoint)
	request.FileName = strings.TrimSpace(request.FileName)
	request.ContentType = strings.TrimSpace(request.ContentType)
	request.Format = strings.TrimSpace(strings.ToLower(request.Format))
	request.StoragePath = strings.TrimSpace(request.StoragePath)
	return request
}

func normalizeUpdateRequest(request UpdateRequest) UpdateRequest {
	request.Label = strings.TrimSpace(request.Label)
	return request
}

func uniqueNonEmpty(ids []string) []string {
	seen := make(map[string]struct{})
	unique := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}
