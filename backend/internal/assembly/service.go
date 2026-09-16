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

	return s.ManifestForConfig(ctx, storedProject.ID, config)
}

func (s *Service) ManifestForConfig(ctx context.Context, projectID string, config catalog.CharacterConfig) (Manifest, error) {
	if err := s.catalog.ValidateConfig(config); err != nil {
		return Manifest{}, fmt.Errorf("%w: %v", ErrInvalidAssembly, err)
	}

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		projectID = "preview"
	}

	characterScale := clamp(float64(config.Scale.MM)/32, 0.75, 1.5)
	manifest := Manifest{
		SchemaVersion:  1,
		ProjectID:      projectID,
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

		assetPoint, err := resolveAssetPoint(asset, placement.AssetPointID)
		if err != nil {
			return Manifest{}, err
		}

		socket := socketTransform(placement.AttachmentPoint, bodyHeight, config.Pose)
		user := userTransform(placement.Transform)
		world := composeWorldTransform(socket, user, assetPoint, characterScale)
		worldBounds := transformGeometryBounds(asset.Metadata.Bounds, world)
		manifest.Placements = append(manifest.Placements, ResolvedPlacement{
			ID:              placement.ID,
			Slot:            placement.Slot,
			AttachmentPoint: placement.AttachmentPoint,
			AssetPointID:    assetPoint.ID,
			AssetPoint:      assetPoint,
			Asset:           assetRef(asset),
			AssetPoints:     asset.AssemblyPoints,
			SocketTransform: socket,
			UserTransform:   user,
			WorldTransform:  world,
			WorldBounds:     worldBounds,
		})
	}
	attachHeadsToBody(manifest.Placements, characterScale)
	manifestBounds := assemblyBoundsAccumulator{}
	for _, placement := range manifest.Placements {
		if placement.WorldBounds != nil {
			manifestBounds.AddBounds(*placement.WorldBounds)
		}
	}
	manifest.Bounds = manifestBounds.Bounds()

	return manifest, nil
}

// A body can expose a head_mount anchor. Resolve it after all placements so a
// head added before the body follows the same attachment as a head added later.
func attachHeadsToBody(placements []ResolvedPlacement, characterScale float64) {
	var neckSocket *Transform
	for _, body := range placements {
		if body.Slot != "body" {
			continue
		}
		for _, point := range body.AssetPoints {
			if point.ID != "head_mount" {
				continue
			}
			position := applyTransform([3]float64(point.Position), body.WorldTransform)
			socket := Transform{Scale: body.WorldTransform.Scale / characterScale}
			for axis := range position {
				socket.Position[axis] = position[axis] / characterScale
				socket.Rotation[axis] = body.WorldTransform.Rotation[axis] + point.Rotation[axis]
			}
			neckSocket = &socket
			break
		}
	}
	if neckSocket == nil {
		return
	}
	for index := range placements {
		head := &placements[index]
		if head.Slot != "head" || head.AttachmentPoint != "head_mount" {
			continue
		}
		head.SocketTransform = *neckSocket
		head.WorldTransform = composeWorldTransform(*neckSocket, head.UserTransform, head.AssetPoint, characterScale)
		head.WorldBounds = transformGeometryBounds(head.Asset.Metadata.Bounds, head.WorldTransform)
	}
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
		Metadata:    asset.Metadata,
		StoragePath: asset.StoragePath,
	}
}

func resolveAssetPoint(asset modelasset.ModelAsset, requestedID string) (modelasset.AssemblyPoint, error) {
	requestedID = strings.TrimSpace(requestedID)
	if requestedID == "" {
		if len(asset.AssemblyPoints) > 0 {
			return asset.AssemblyPoints[0], nil
		}
		return modelasset.AssemblyPoint{
			ID:       asset.AttachmentPoint,
			Label:    asset.AttachmentPoint,
			Position: modelasset.Vector3{0, 0, 0},
			Rotation: modelasset.Vector3{0, 0, 0},
		}, nil
	}

	for _, point := range asset.AssemblyPoints {
		if point.ID == requestedID {
			return point, nil
		}
	}

	return modelasset.AssemblyPoint{}, fmt.Errorf("%w: asset %q does not expose assembly point %q", ErrInvalidAssembly, asset.ID, requestedID)
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

func composeWorldTransform(socket Transform, user Transform, assetPoint modelasset.AssemblyPoint, characterScale float64) Transform {
	scale := socket.Scale * user.Scale * characterScale
	rotation := [3]float64{
		round(socket.Rotation[0] + user.Rotation[0] - assetPoint.Rotation[0]),
		round(socket.Rotation[1] + user.Rotation[1] - assetPoint.Rotation[1]),
		round(socket.Rotation[2] + user.Rotation[2] - assetPoint.Rotation[2]),
	}
	target := [3]float64{
		(socket.Position[0] + user.Position[0]) * characterScale,
		(socket.Position[1] + user.Position[1]) * characterScale,
		(socket.Position[2] + user.Position[2]) * characterScale,
	}
	anchorOffset := rotateVector([3]float64{
		assetPoint.Position[0] * scale,
		assetPoint.Position[1] * scale,
		assetPoint.Position[2] * scale,
	}, rotation)

	return Transform{
		Position: [3]float64{
			round(target[0] - anchorOffset[0]),
			round(target[1] - anchorOffset[1]),
			round(target[2] - anchorOffset[2]),
		},
		Rotation: rotation,
		Scale:    round(scale),
	}
}

func transformGeometryBounds(bounds *modelasset.GeometryBounds, transform Transform) *modelasset.GeometryBounds {
	if bounds == nil {
		return nil
	}

	accumulator := assemblyBoundsAccumulator{}
	for _, corner := range geometryBoundsCorners(*bounds) {
		accumulator.AddVector(applyTransform(corner, transform))
	}
	return accumulator.Bounds()
}

func geometryBoundsCorners(bounds modelasset.GeometryBounds) [8][3]float64 {
	min := bounds.Min
	max := bounds.Max
	return [8][3]float64{
		{min[0], min[1], min[2]},
		{min[0], min[1], max[2]},
		{min[0], max[1], min[2]},
		{min[0], max[1], max[2]},
		{max[0], min[1], min[2]},
		{max[0], min[1], max[2]},
		{max[0], max[1], min[2]},
		{max[0], max[1], max[2]},
	}
}

func applyTransform(vector [3]float64, transform Transform) [3]float64 {
	x := vector[0] * transform.Scale
	y := vector[1] * transform.Scale
	z := vector[2] * transform.Scale

	x, y, z = rotateX(x, y, z, degreesToRadians(transform.Rotation[0]))
	x, y, z = rotateY(x, y, z, degreesToRadians(transform.Rotation[1]))
	x, y, z = rotateZ(x, y, z, degreesToRadians(transform.Rotation[2]))

	return [3]float64{
		round(x + transform.Position[0]),
		round(y + transform.Position[1]),
		round(z + transform.Position[2]),
	}
}

type assemblyBoundsAccumulator struct {
	min modelasset.Vector3
	max modelasset.Vector3
	has bool
}

func (bounds *assemblyBoundsAccumulator) AddBounds(next modelasset.GeometryBounds) {
	bounds.AddVector([3]float64{next.Min[0], next.Min[1], next.Min[2]})
	bounds.AddVector([3]float64{next.Max[0], next.Max[1], next.Max[2]})
}

func (bounds *assemblyBoundsAccumulator) AddVector(vector [3]float64) {
	if !bounds.has {
		bounds.min = modelasset.Vector3{vector[0], vector[1], vector[2]}
		bounds.max = modelasset.Vector3{vector[0], vector[1], vector[2]}
		bounds.has = true
		return
	}
	for index := 0; index < 3; index++ {
		bounds.min[index] = math.Min(bounds.min[index], vector[index])
		bounds.max[index] = math.Max(bounds.max[index], vector[index])
	}
}

func (bounds assemblyBoundsAccumulator) Bounds() *modelasset.GeometryBounds {
	if !bounds.has {
		return nil
	}

	size := modelasset.Vector3{
		round(bounds.max[0] - bounds.min[0]),
		round(bounds.max[1] - bounds.min[1]),
		round(bounds.max[2] - bounds.min[2]),
	}
	center := modelasset.Vector3{
		round((bounds.min[0] + bounds.max[0]) / 2),
		round((bounds.min[1] + bounds.max[1]) / 2),
		round((bounds.min[2] + bounds.max[2]) / 2),
	}

	return &modelasset.GeometryBounds{
		Min:    roundVector(bounds.min),
		Max:    roundVector(bounds.max),
		Size:   size,
		Center: center,
	}
}

func roundVector(vector modelasset.Vector3) modelasset.Vector3 {
	return modelasset.Vector3{
		round(vector[0]),
		round(vector[1]),
		round(vector[2]),
	}
}

func rotateVector(vector [3]float64, rotation [3]float64) [3]float64 {
	x := vector[0]
	y := vector[1]
	z := vector[2]

	x, y, z = rotateX(x, y, z, degreesToRadians(rotation[0]))
	x, y, z = rotateY(x, y, z, degreesToRadians(rotation[1]))
	x, y, z = rotateZ(x, y, z, degreesToRadians(rotation[2]))

	return [3]float64{x, y, z}
}

func rotateX(x float64, y float64, z float64, angle float64) (float64, float64, float64) {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	return x, y*cos - z*sin, y*sin + z*cos
}

func rotateY(x float64, y float64, z float64, angle float64) (float64, float64, float64) {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	return x*cos + z*sin, y, -x*sin + z*cos
}

func rotateZ(x float64, y float64, z float64, angle float64) (float64, float64, float64) {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	return x*cos - y*sin, x*sin + y*cos, z
}

func degreesToRadians(value float64) float64 {
	return value * math.Pi / 180
}

func clamp(value float64, min float64, max float64) float64 {
	return math.Min(max, math.Max(min, value))
}

func round(value float64) float64 {
	return math.Round(value*10000) / 10000
}
