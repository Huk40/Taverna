package assembly

import (
	"time"

	"taverna/backend/internal/modelasset"
)

type Manifest struct {
	SchemaVersion  int                 `json:"schema_version"`
	ProjectID      string              `json:"project_id"`
	GeneratedAt    time.Time           `json:"generated_at"`
	ScaleMM        int                 `json:"scale_mm"`
	CharacterScale float64             `json:"character_scale"`
	Unit           string              `json:"unit"`
	Selections     map[string]string   `json:"selections"`
	Placements     []ResolvedPlacement `json:"placements"`
	Warnings       []string            `json:"warnings"`
}

type ResolvedPlacement struct {
	ID              string                     `json:"id"`
	Slot            string                     `json:"slot"`
	AttachmentPoint string                     `json:"attachment_point"`
	AssetPointID    string                     `json:"asset_point_id"`
	AssetPoint      modelasset.AssemblyPoint   `json:"asset_point"`
	Asset           ModelAssetRef              `json:"asset"`
	AssetPoints     []modelasset.AssemblyPoint `json:"asset_points"`
	SocketTransform Transform                  `json:"socket_transform"`
	UserTransform   Transform                  `json:"user_transform"`
	WorldTransform  Transform                  `json:"world_transform"`
}

type ModelAssetRef struct {
	ID          string                      `json:"id"`
	Label       string                      `json:"label"`
	FileName    string                      `json:"file_name"`
	ContentType string                      `json:"content_type"`
	Format      string                      `json:"format"`
	SizeBytes   int64                       `json:"size_bytes"`
	URL         string                      `json:"url"`
	Metadata    modelasset.GeometryMetadata `json:"metadata"`
	StoragePath string                      `json:"-"`
}

type Transform struct {
	Position [3]float64 `json:"position"`
	Rotation [3]float64 `json:"rotation"`
	Scale    float64    `json:"scale"`
}
