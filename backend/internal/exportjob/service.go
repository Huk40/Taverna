package exportjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"taverna/backend/internal/assembly"
)

type JobStore interface {
	Insert(context.Context, string, string, string) (ExportJob, error)
	Complete(context.Context, string, json.RawMessage) (ExportJob, error)
	Fail(context.Context, string, string) error
}

type Builder interface {
	Build(context.Context, string, assembly.Manifest, string) (ArtifactResult, error)
	Remove(ArtifactResult) error
}

type Service struct {
	store   JobStore
	builder Builder
}

func NewService(store JobStore, builder Builder) *Service { return &Service{store, builder} }

func (s *Service) Create(ctx context.Context, projectID string, request CreateRequest, manifest assembly.Manifest) (job ExportJob, err error) {
	target := normalizeTarget(request.Target)
	format := normalizeFormat(request.Format, target)
	if !allowedTarget(target) || !allowedFormat(format) {
		return ExportJob{}, fmt.Errorf("%w: only OBJ and STL file exports are supported", ErrInvalidRequest)
	}
	if len(manifest.Placements) == 0 {
		return ExportJob{}, fmt.Errorf("%w: assembly contains no assets", ErrInvalidRequest)
	}
	job, err = s.store.Insert(ctx, projectID, target, format)
	if err != nil {
		return job, err
	}
	jobID := job.ID
	var file ArtifactResult
	completed := false
	defer func() {
		if completed {
			return
		}
		// Request cancellation must not prevent recording a terminal status.
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if file.StoragePath != "" {
			err = errors.Join(err, s.builder.Remove(file))
		}
		if failure := s.store.Fail(cleanup, jobID, "Export failed; see server logs for details"); failure != nil {
			err = errors.Join(err, fmt.Errorf("persist failed export: %w", failure))
		}
	}()
	file, err = s.builder.Build(ctx, jobID, manifest, format)
	if err != nil {
		return job, fmt.Errorf("build artifact: %w", err)
	}
	if file.URL == "" || file.SizeBytes <= 0 || len(file.SkippedAssets) != 0 {
		return job, errors.New("builder returned an incomplete artifact")
	}
	raw, err := buildArtifact(projectID, target, format, manifest, file)
	if err != nil {
		return job, err
	}
	job, err = s.store.Complete(ctx, jobID, raw)
	completed = err == nil
	return job, err
}

func buildArtifact(projectID, target, format string, manifest assembly.Manifest, file ArtifactResult) (json.RawMessage, error) {
	ids, points := []string{}, []string{}
	for _, p := range manifest.Placements {
		ids = append(ids, p.Asset.ID)
		points = append(points, p.Slot+"/"+p.AttachmentPoint)
	}
	return json.Marshal(map[string]any{
		"kind": "export_job_artifact", "schema_version": 1, "source_project_id": projectID,
		"target": target, "format": format, "assembly_manifest": manifest, "artifact_file": file,
		"assembly":        map[string]any{"placement_count": len(ids), "asset_ids": ids, "attachment_points": points, "status": "assembly_plan_ready"},
		"print_readiness": map[string]any{"status": "geometry_exported", "warnings": []string{"watertightness_and_printability_not_checked"}},
	})
}
