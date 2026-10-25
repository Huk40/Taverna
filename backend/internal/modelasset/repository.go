package modelasset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"taverna/backend/internal/httpx"
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
	metadata, err := json.Marshal(request.Metadata)
	if err != nil {
		return ModelAsset{}, fmt.Errorf("marshal geometry metadata: %w", err)
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO model_assets (
			id, label, slot, attachment_point, file_name, content_type, format, size_bytes, storage_path, assembly_points, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11::jsonb)
		RETURNING id::text, label, slot, attachment_point, file_name, content_type, format, size_bytes, storage_path, assembly_points, metadata, created_at, updated_at
	`, request.ID, request.Label, request.Slot, request.AttachmentPoint, request.FileName, request.ContentType, request.Format, request.SizeBytes, request.StoragePath, string(points), string(metadata))

	return scanAsset(row)
}

func (r *Repository) List(ctx context.Context, slot string, pages ...httpx.Page) ([]ModelAsset, error) {
	page := httpx.Page{Limit: 50}
	if len(pages) > 0 {
		page = pages[0]
	}
	var (
		rows pgx.Rows
		err  error
	)

	if slot == "" {
		rows, err = r.pool.Query(ctx, `
			SELECT id::text, label, slot, attachment_point, file_name, content_type, format, size_bytes, storage_path, assembly_points, metadata, created_at, updated_at
			FROM model_assets
			ORDER BY updated_at DESC, id DESC LIMIT $1 OFFSET $2
		`, page.Limit, page.Offset)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT id::text, label, slot, attachment_point, file_name, content_type, format, size_bytes, storage_path, assembly_points, metadata, created_at, updated_at
			FROM model_assets
			WHERE slot = $1
			ORDER BY updated_at DESC, id DESC LIMIT $2 OFFSET $3
		`, strings.TrimSpace(slot), page.Limit, page.Offset)
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
		SELECT id::text, label, slot, attachment_point, file_name, content_type, format, size_bytes, storage_path, assembly_points, metadata, created_at, updated_at
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
		RETURNING id::text, label, slot, attachment_point, file_name, content_type, format, size_bytes, storage_path, assembly_points, metadata, created_at, updated_at
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

func (r *Repository) ReferencingProjects(ctx context.Context, id string) ([]ProjectReference, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id::text, name
		FROM projects
		WHERE EXISTS (
			SELECT 1
			FROM jsonb_array_elements(
				CASE
					WHEN jsonb_typeof(config->'assembly'->'placements') = 'array'
					THEN config->'assembly'->'placements'
					ELSE '[]'::jsonb
				END
			) AS placement
			WHERE placement->>'asset_id' = $1
		)
		ORDER BY updated_at DESC, created_at DESC
		LIMIT 20
	`, id)
	if err != nil {
		return nil, fmt.Errorf("query project references for model asset: %w", err)
	}
	defer rows.Close()

	references := make([]ProjectReference, 0)
	for rows.Next() {
		var reference ProjectReference
		if err := rows.Scan(&reference.ID, &reference.Name); err != nil {
			return nil, fmt.Errorf("scan model asset project reference: %w", err)
		}
		references = append(references, reference)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read model asset project references: %w", err)
	}

	return references, nil
}

func (r *Repository) MissingAnchorReferences(ctx context.Context, assetID string, allowedPointIDs []string) ([]AnchorReference, error) {
	assetID = strings.TrimSpace(assetID)
	if assetID == "" {
		return nil, nil
	}
	allowed := uniqueNonEmpty(allowedPointIDs)

	rows, err := r.pool.Query(ctx, `
		SELECT
			p.id::text,
			p.name,
			COALESCE(placement->>'id', ''),
			COALESCE(placement->>'asset_point_id', '')
		FROM projects p
		CROSS JOIN LATERAL jsonb_array_elements(
			CASE
				WHEN jsonb_typeof(p.config->'assembly'->'placements') = 'array'
				THEN p.config->'assembly'->'placements'
				ELSE '[]'::jsonb
			END
		) AS placement
		WHERE placement->>'asset_id' = $1
			AND COALESCE(NULLIF(BTRIM(placement->>'asset_point_id'), ''), '') <> ''
			AND NOT (placement->>'asset_point_id' = ANY($2::text[]))
		ORDER BY p.updated_at DESC, p.created_at DESC
		LIMIT 20
	`, assetID, allowed)
	if err != nil {
		return nil, fmt.Errorf("query project references for missing model asset anchors: %w", err)
	}
	defer rows.Close()

	references := make([]AnchorReference, 0)
	for rows.Next() {
		var reference AnchorReference
		if err := rows.Scan(&reference.ProjectID, &reference.ProjectName, &reference.PlacementID, &reference.AssetPointID); err != nil {
			return nil, fmt.Errorf("scan missing model asset anchor reference: %w", err)
		}
		references = append(references, reference)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read missing model asset anchor references: %w", err)
	}

	return references, nil
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
		asset       ModelAsset
		rawPoints   []byte
		rawMetadata []byte
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
		&rawMetadata,
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
	if len(rawMetadata) > 0 {
		if err := json.Unmarshal(rawMetadata, &asset.Metadata); err != nil {
			return ModelAsset{}, fmt.Errorf("decode model asset metadata: %w", err)
		}
	}
	asset.Metadata = normalizeGeometryMetadata(asset.Metadata)
	asset.URL = "/api/model-assets/" + asset.ID + "/file"

	return asset, nil
}

func normalizeCreateRequest(request CreateRequest) CreateRequest {
	if request.AssemblyPoints == nil {
		request.AssemblyPoints = []AssemblyPoint{}
	}
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
	request.Metadata = normalizeGeometryMetadata(request.Metadata)
	return request
}

func normalizeUpdateRequest(request UpdateRequest) UpdateRequest {
	request.Label = strings.TrimSpace(request.Label)
	return request
}

func normalizeGeometryMetadata(metadata GeometryMetadata) GeometryMetadata {
	metadata.Status = strings.TrimSpace(metadata.Status)
	if metadata.Status == "" {
		metadata.Status = "unknown"
	}
	metadata.Source = strings.TrimSpace(metadata.Source)
	if metadata.Warnings == nil {
		metadata.Warnings = []string{}
	}
	return metadata
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
