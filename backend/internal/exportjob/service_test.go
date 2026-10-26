package exportjob

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"taverna/backend/internal/assembly"
)

type memoryJobs struct {
	inserted, failed, completed bool
	completeErr, failErr        error
}

func (m *memoryJobs) Insert(context.Context, string, string, string) (ExportJob, error) {
	m.inserted = true
	return ExportJob{ID: "job"}, nil
}
func (m *memoryJobs) Complete(_ context.Context, _ string, raw json.RawMessage) (ExportJob, error) {
	m.completed = true
	return ExportJob{ID: "job", Status: StatusCompleted, Artifact: raw}, m.completeErr
}
func (m *memoryJobs) Fail(ctx context.Context, _ string, _ string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	m.failed = true
	return m.failErr
}

type testBuilder struct {
	file    ArtifactResult
	err     error
	removed bool
	cancel  context.CancelFunc
}

func (b *testBuilder) Build(context.Context, string, assembly.Manifest, string) (ArtifactResult, error) {
	if b.cancel != nil {
		b.cancel()
	}
	return b.file, b.err
}
func (b *testBuilder) Remove(ArtifactResult) error { b.removed = true; return nil }

func TestServiceRejectsUnsupportedFormatBeforeInsert(t *testing.T) {
	for _, format := range []string{"glb", "3mf", "fbx"} {
		store := &memoryJobs{}
		_, err := NewService(store, &testBuilder{}).Create(context.Background(), "p", CreateRequest{Format: format}, assembly.Manifest{Placements: []assembly.ResolvedPlacement{{ID: "one"}}})
		if !errors.Is(err, ErrInvalidRequest) || store.inserted {
			t.Fatalf("format %s: err=%v inserted=%v", format, err, store.inserted)
		}
	}
}

func TestServiceRecordsFailuresAndCleansFiles(t *testing.T) {
	for _, scenario := range []string{"build", "cancellation", "missing file", "partial", "complete"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			store := &memoryJobs{}
			builder := &testBuilder{file: ArtifactResult{URL: "/artifact", StoragePath: "/file", SizeBytes: 100}}
			switch scenario {
			case "build":
				builder.err = errors.New("disk full")
			case "cancellation":
				builder.cancel = cancel
				builder.err = context.Canceled
			case "missing file":
				builder.file.URL = ""
			case "partial":
				builder.file.SkippedAssets = []string{"missing"}
			case "complete":
				store.completeErr = errors.New("database unavailable")
			}
			_, err := NewService(store, builder).Create(ctx, "p", CreateRequest{Format: "stl"}, assembly.Manifest{Placements: []assembly.ResolvedPlacement{{ID: "one"}}})
			if err == nil || !store.failed || !builder.removed {
				t.Fatalf("err=%v failed=%v removed=%v", err, store.failed, builder.removed)
			}
		})
	}
}

func TestServiceCompletesOnlyWithArtifact(t *testing.T) {
	store := &memoryJobs{}
	builder := &testBuilder{file: ArtifactResult{URL: "/artifact", StoragePath: "/private/path", SizeBytes: 100}}
	job, err := NewService(store, builder).Create(context.Background(), "p", CreateRequest{Format: "obj"}, assembly.Manifest{Placements: []assembly.ResolvedPlacement{{ID: "one"}}})
	if err != nil || job.Status != StatusCompleted || store.failed || builder.removed {
		t.Fatalf("job=%+v err=%v", job, err)
	}
	var artifact struct {
		File map[string]any `json:"artifact_file"`
	}
	if err := json.Unmarshal(job.Artifact, &artifact); err != nil {
		t.Fatal(err)
	}
	if _, ok := artifact.File["storage_path"]; ok {
		t.Fatal("private filesystem path leaked")
	}
}

func TestServiceRejectsEmptySelectionBeforeInsert(t *testing.T) {
	store := &memoryJobs{}
	_, err := NewService(store, &testBuilder{}).Create(context.Background(), "p", CreateRequest{}, assembly.Manifest{})
	if !errors.Is(err, ErrInvalidRequest) || store.inserted {
		t.Fatalf("err=%v inserted=%v", err, store.inserted)
	}
}
