package exportjob

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"taverna/backend/internal/assembly"
	"taverna/backend/internal/modelasset"
)

type contextWriter struct {
	ctx    context.Context
	target io.Writer
}

func (w contextWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	return w.target.Write(p)
}

func (b *ArtifactBuilder) Remove(file ArtifactResult) error {
	rel, err := filepath.Rel(b.storageDir, file.StoragePath)
	if err != nil || filepath.IsAbs(rel) || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("artifact outside storage directory")
	}
	err = os.Remove(file.StoragePath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func visitSTLPlacementContext(ctx context.Context, write triangleWriter, p assembly.ResolvedPlacement) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	checked := func(a, b, c [3]float64) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return write(a, b, c)
	}
	var triangles []modelasset.Triangle
	var err error
	switch p.Asset.Format {
	case modelasset.FormatGLB:
		triangles, err = modelasset.ReadGLBTrianglesContext(ctx, p.Asset.StoragePath)
	case modelasset.FormatGLTF:
		triangles, err = modelasset.ReadGLTFTrianglesContext(ctx, p.Asset.StoragePath)
	case modelasset.Format3MF:
		triangles, err = modelasset.Read3MFTrianglesContext(ctx, p.Asset.StoragePath)
	default:
		return visitSTLPlacement(checked, p)
	}
	if err != nil {
		return 0, err
	}
	for _, tri := range triangles {
		if err := checked(applyTransform(vector3ToArray(tri[0]), p.WorldTransform, true), applyTransform(vector3ToArray(tri[1]), p.WorldTransform, true), applyTransform(vector3ToArray(tri[2]), p.WorldTransform, true)); err != nil {
			return 0, err
		}
	}
	return len(triangles), nil
}
