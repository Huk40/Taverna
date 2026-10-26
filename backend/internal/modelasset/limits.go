package modelasset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	MaxGeometryTriangles       = 250_000
	maxGeometryVertices        = 1_000_000
	maxGeometryBytes     int64 = 64 << 20
)

func primitiveTriangleCount(indices, mode int) int {
	if mode == 5 || mode == 6 {
		return max(0, indices-2)
	}
	if mode == 4 {
		return indices / 3
	}
	return 0
}

type contextReader struct {
	ctx    context.Context
	source io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.source.Read(p)
}

func readGeometryFile(ctx context.Context, path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readBoundedGeometry(contextReader{ctx, file}, maxGeometryBytes)
}

func readBoundedGeometry(source io.Reader, limit int64) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(source, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, errors.New("geometry exceeds decompressed size limit")
	}
	return raw, nil
}

// Validate the entire accessor before allocating or multiplying untrusted counts.
func validateAccessor(count, stride, elementSize, start, end int) error {
	if count <= 0 || count > maxGeometryVertices*3 || stride < elementSize || start < 0 || end < start || end-start < elementSize || count-1 > (end-start-elementSize)/stride {
		return fmt.Errorf("accessor count or byte range exceeds geometry limits")
	}
	return nil
}

func inspectTriangleGeometry(ctx context.Context, path, format string) (GeometryMetadata, error) {
	var triangles []Triangle
	var err error
	if format == FormatGLTF {
		triangles, err = ReadGLTFTrianglesContext(ctx, path)
	} else {
		triangles, err = Read3MFTrianglesContext(ctx, path)
	}
	if err != nil {
		return GeometryMetadata{}, err
	}
	bounds := geometryBoundsAccumulator{}
	for _, tri := range triangles {
		for _, v := range tri {
			bounds.Add(v)
		}
	}
	return finishGeometryMetadata(format, len(triangles)*3, len(triangles), len(triangles), bounds, []string{}), nil
}
