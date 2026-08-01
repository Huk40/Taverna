package assembly

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"taverna/backend/internal/catalog"
	"taverna/backend/internal/modelasset"
	"taverna/backend/internal/project"
)

var ErrProjectNotFound = errors.New("project not found")
var ErrInvalidAssembly = errors.New("invalid assembly")

type Service struct {
	projects *project.Repository
	assets   *modelasset.Repository
	catalog  *catalog.Service
}

func NewService(projects *project.Repository, assets *modelasset.Repository, catalogService *catalog.Service) *Service {
	return &Service{projects: projects, assets: assets, catalog: catalogService}
}

func (s *Service) Manifest(ctx context.Context, projectID string) (Manifest, error) {
	storedProject, err := s.projects.Get(ctx, projectID)
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			return Manifest{}, ErrProjectNotFound
		}
		return Manifest{}, fmt.Errorf("get project: %w", err)
	}

	config, err := s.catalog.DecodeRawConfig(storedProject.Config)
	if err != nil {
		return Manifest{}, fmt.Errorf("%w: %v", ErrInvalidAssembly, err)
	}
	if err := s.catalog.ValidateConfig(config); err != nil {
		return Manifest{}, fmt.Errorf("%w: %v", ErrInvalidAssembly, err)
	}

	characterScale := clamp(float64(config.Scale.MM)/32, 0.75, 1.5)
	manifest := Manifest{
		SchemaVersion:  1,
		ProjectID:      storedProject.ID,
		GeneratedAt:    time.Now().UTC(),
		ScaleMM:        config.Scale.MM,
		CharacterScale: characterScale,
		Unit:           "three_scene_unit",
		Selections:     selectionsFor(config),
		Placements:     make([]ResolvedPlacement, 0, len(config.Assembly.Placements)),
		Warnings:       []string{},
	}

	bodyHeight := bodyHeightFor(config.Body)
	for _, placement := range config.Assembly.Placements {
		asset, err := s.assets.Get(ctx, placement.AssetID)
		if err != nil {
			if errors.Is(err, modelasset.ErrNotFound) {
				return Manifest{}, fmt.Errorf("%w: missing model asset %q", ErrInvalidAssembly, placement.AssetID)
			}
			return Manifest{}, fmt.Errorf("get model asset %q: %w", placement.AssetID, err)
		}
		if asset.Slot != placement.Slot {
			return Manifest{}, fmt.Errorf("%w: asset %q belongs to slot %q, placement uses slot %q", ErrInvalidAssembly, asset.ID, asset.Slot, placement.Slot)
		}

		socket := socketTransform(placement.AttachmentPoint, bodyHeight, config.Pose)
		user := userTransform(placement.Transform)
		world := composeWorldTransform(socket, user, characterScale)
		if asset.Format == modelasset.Format3MF {
			manifest.Warnings = append(manifest.Warnings, fmt.Sprintf("asset %s is 3MF and currently resolves as a manifest-only placement", asset.ID))
		}

		manifest.Placements = append(manifest.Placements, ResolvedPlacement{
			ID:              placement.ID,
			Slot:            placement.Slot,
			AttachmentPoint: placement.AttachmentPoint,
			Asset:           assetRef(asset),
			AssetPoints:     asset.AssemblyPoints,
			SocketTransform: socket,
			UserTransform:   user,
			WorldTransform:  world,
		})
	}

	return manifest, nil
}

func assetRef(asset modelasset.ModelAsset) ModelAssetRef {
	return ModelAssetRef{
		ID:          asset.ID,
		Label:       asset.Label,
		FileName:    asset.FileName,
		ContentType: asset.ContentType,
		Format:      asset.Format,
		SizeBytes:   asset.SizeBytes,
		URL:         asset.URL,
		StoragePath: asset.StoragePath,
	}
}

func selectionsFor(config catalog.CharacterConfig) map[string]string {
	return map[string]string{
		"species":        config.Species,
		"body":           config.Body,
		"head":           config.Head,
		"clothing":       config.Clothing,
		"gear":           config.Gear,
		"pose":           config.Pose,
		"stage":          config.Stage,
		"base":           config.Base,
		"product_target": config.ProductTarget,
	}
}

func bodyHeightFor(body string) float64 {
	switch {
	case strings.Contains(body, "compact"):
		return 1.28
	case strings.Contains(body, "heroic"):
		return 1.78
	default:
		return 1.62
	}
}

func socketTransform(point string, bodyHeight float64, pose string) Transform {
	guardTilt := 0.0
	if strings.Contains(pose, "guard") {
		guardTilt = 25.78
	}

	switch point {
	case "body_core":
		return Transform{Position: [3]float64{0, 0.96, 0}, Rotation: [3]float64{0, 0, 0}, Scale: 1}
	case "head_mount":
		return Transform{Position: [3]float64{0, bodyHeight + 1.02, 0}, Rotation: [3]float64{0, 0, 0}, Scale: 1}
	case "torso_mount":
		return Transform{Position: [3]float64{0, 1.18, 0.24}, Rotation: [3]float64{0, 0, 0}, Scale: 1}
	case "back_mount":
		return Transform{Position: [3]float64{0, 1.16, -0.38}, Rotation: [3]float64{5.73, 180, 0}, Scale: 1}
	case "left_hand":
		return Transform{Position: [3]float64{-0.78, 1.08, 0.04}, Rotation: [3]float64{6.88, 0, 11.46 + guardTilt}, Scale: 1}
	case "right_hand":
		return Transform{Position: [3]float64{0.78, 1.08, 0.04}, Rotation: [3]float64{6.88, 0, -11.46 - guardTilt}, Scale: 1}
	case "stage_anchor":
		return Transform{Position: [3]float64{0, 0.2, 0.62}, Rotation: [3]float64{0, 0, 0}, Scale: 1}
	case "base_mount":
		return Transform{Position: [3]float64{0, 0.22, 0}, Rotation: [3]float64{0, 0, 0}, Scale: 1}
	default:
		return Transform{Position: [3]float64{0, 1, 0}, Rotation: [3]float64{0, 0, 0}, Scale: 1}
	}
}

func userTransform(transform catalog.AssemblyTransform) Transform {
	scale := transform.Scale
	if scale == 0 {
		scale = 1
	}
	return Transform{
		Position: [3]float64{transform.Position[0], transform.Position[1], transform.Position[2]},
		Rotation: [3]float64{transform.Rotation[0], transform.Rotation[1], transform.Rotation[2]},
		Scale:    scale,
	}
}

func composeWorldTransform(socket Transform, user Transform, characterScale float64) Transform {
	return Transform{
		Position: [3]float64{
			round((socket.Position[0] + user.Position[0]) * characterScale),
			round((socket.Position[1] + user.Position[1]) * characterScale),
			round((socket.Position[2] + user.Position[2]) * characterScale),
		},
		Rotation: [3]float64{
			round(socket.Rotation[0] + user.Rotation[0]),
			round(socket.Rotation[1] + user.Rotation[1]),
			round(socket.Rotation[2] + user.Rotation[2]),
		},
		Scale: round(socket.Scale * user.Scale * characterScale),
	}
}

func clamp(value float64, min float64, max float64) float64 {
	return math.Min(max, math.Max(min, value))
}

func round(value float64) float64 {
	return math.Round(value*10000) / 10000
}
