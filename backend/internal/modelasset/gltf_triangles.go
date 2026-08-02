package modelasset

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
)

func ReadGLTFTriangles(path string) ([]Triangle, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read GLTF triangles: %w", err)
	}

	var document glbDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("parse GLTF JSON: %w", err)
	}

	buffers, err := readEmbeddedGLTFBuffers(document)
	if err != nil {
		return nil, err
	}

	triangles, err := readBufferedDocumentTriangles(document, buffers, "GLTF")
	if err != nil {
		return nil, err
	}
	if len(triangles) == 0 {
		return nil, fmt.Errorf("GLTF contains no triangle primitives")
	}
	return triangles, nil
}

func inspectGLTFGeometry(path string) (GeometryMetadata, error) {
	triangles, err := ReadGLTFTriangles(path)
	if err != nil {
		return GeometryMetadata{}, err
	}

	bounds := geometryBoundsAccumulator{}
	for _, triangle := range triangles {
		for _, vertex := range triangle {
			bounds.Add(vertex)
		}
	}

	triangleCount := len(triangles)
	vertexCount := triangleCount * 3
	return finishGeometryMetadata("gltf", vertexCount, triangleCount, triangleCount, bounds, []string{}), nil
}

func readEmbeddedGLTFBuffers(document glbDocument) ([][]byte, error) {
	if len(document.Buffers) == 0 {
		return nil, fmt.Errorf("GLTF contains no buffers")
	}

	buffers := make([][]byte, 0, len(document.Buffers))
	for index, buffer := range document.Buffers {
		uri := strings.TrimSpace(buffer.URI)
		if uri == "" {
			return nil, fmt.Errorf("GLTF buffer %d has no URI; only embedded data URI buffers are supported", index)
		}
		if !strings.HasPrefix(strings.ToLower(uri), "data:") {
			return nil, fmt.Errorf("GLTF buffer %d uses external URI %q; only embedded data URI buffers are supported", index, uri)
		}

		metadata, encoded, ok := strings.Cut(uri, ",")
		if !ok || !strings.Contains(strings.ToLower(metadata), ";base64") {
			return nil, fmt.Errorf("GLTF buffer %d must be a base64 data URI", index)
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
		if err != nil {
			return nil, fmt.Errorf("decode GLTF buffer %d data URI: %w", index, err)
		}
		if buffer.ByteLength > 0 && len(decoded) < buffer.ByteLength {
			return nil, fmt.Errorf("GLTF buffer %d byteLength %d exceeds decoded length %d", index, buffer.ByteLength, len(decoded))
		}
		buffers = append(buffers, decoded)
	}
	return buffers, nil
}

func readBufferedDocumentTriangles(document glbDocument, buffers [][]byte, source string) ([]Triangle, error) {
	triangles := make([]Triangle, 0)
	for meshIndex, mesh := range document.Meshes {
		for primitiveIndex, primitive := range mesh.Primitives {
			positionAccessorIndex, ok := primitive.Attributes["POSITION"]
			if !ok {
				return nil, fmt.Errorf("%s mesh %d primitive %d has no POSITION attribute", source, meshIndex, primitiveIndex)
			}

			positions, err := readBufferedPositionAccessor(document, buffers, positionAccessorIndex, source)
			if err != nil {
				return nil, fmt.Errorf("read %s mesh %d primitive %d positions: %w", source, meshIndex, primitiveIndex, err)
			}

			indices, err := readBufferedPrimitiveIndices(document, buffers, primitive, len(positions), source)
			if err != nil {
				return nil, fmt.Errorf("read %s mesh %d primitive %d indices: %w", source, meshIndex, primitiveIndex, err)
			}

			triangles = append(triangles, trianglesForGLBPrimitive(positions, indices, primitiveMode(primitive))...)
		}
	}
	return triangles, nil
}

func readBufferedPositionAccessor(document glbDocument, buffers [][]byte, accessorIndex int, source string) ([]Vector3, error) {
	accessor, err := glbAccessorAt(document, accessorIndex)
	if err != nil {
		return nil, err
	}
	if accessor.ComponentType != 5126 || accessor.Type != "VEC3" {
		return nil, fmt.Errorf("POSITION accessor %d must be FLOAT VEC3", accessorIndex)
	}
	if accessor.Count <= 0 {
		return nil, fmt.Errorf("POSITION accessor %d has no vertices", accessorIndex)
	}

	view, buffer, start, err := bufferedAccessorRange(document, buffers, accessor, accessorIndex, source)
	if err != nil {
		return nil, err
	}
	stride := view.ByteStride
	if stride == 0 {
		stride = 12
	}
	if stride < 12 {
		return nil, fmt.Errorf("POSITION accessor %d has byteStride smaller than VEC3 FLOAT", accessorIndex)
	}

	positions := make([]Vector3, 0, accessor.Count)
	viewEnd := view.ByteOffset + view.ByteLength
	for index := 0; index < accessor.Count; index++ {
		offset := start + index*stride
		if offset+12 > viewEnd || offset+12 > len(buffer) {
			return nil, fmt.Errorf("POSITION accessor %d reads past bufferView", accessorIndex)
		}
		positions = append(positions, Vector3{
			float64(math.Float32frombits(binary.LittleEndian.Uint32(buffer[offset : offset+4]))),
			float64(math.Float32frombits(binary.LittleEndian.Uint32(buffer[offset+4 : offset+8]))),
			float64(math.Float32frombits(binary.LittleEndian.Uint32(buffer[offset+8 : offset+12]))),
		})
	}

	return positions, nil
}

func readBufferedPrimitiveIndices(document glbDocument, buffers [][]byte, primitive glbPrimitive, positionCount int, source string) ([]int, error) {
	if primitive.Indices == nil {
		indices := make([]int, positionCount)
		for index := range indices {
			indices[index] = index
		}
		return indices, nil
	}

	accessorIndex := *primitive.Indices
	accessor, err := glbAccessorAt(document, accessorIndex)
	if err != nil {
		return nil, err
	}
	if accessor.Type != "SCALAR" {
		return nil, fmt.Errorf("index accessor %d must be SCALAR", accessorIndex)
	}
	if accessor.Count <= 0 {
		return nil, fmt.Errorf("index accessor %d has no values", accessorIndex)
	}

	componentSize, err := glbIndexComponentSize(accessor.ComponentType)
	if err != nil {
		return nil, fmt.Errorf("index accessor %d: %w", accessorIndex, err)
	}
	view, buffer, start, err := bufferedAccessorRange(document, buffers, accessor, accessorIndex, source)
	if err != nil {
		return nil, err
	}
	stride := view.ByteStride
	if stride == 0 {
		stride = componentSize
	}
	if stride < componentSize {
		return nil, fmt.Errorf("index accessor %d has byteStride smaller than component size", accessorIndex)
	}

	indices := make([]int, 0, accessor.Count)
	viewEnd := view.ByteOffset + view.ByteLength
	for index := 0; index < accessor.Count; index++ {
		offset := start + index*stride
		if offset+componentSize > viewEnd || offset+componentSize > len(buffer) {
			return nil, fmt.Errorf("index accessor %d reads past bufferView", accessorIndex)
		}
		value := glbIndexValue(buffer[offset:offset+componentSize], accessor.ComponentType)
		if value < 0 || value >= positionCount {
			return nil, fmt.Errorf("index %d references vertex %d outside POSITION count %d", index, value, positionCount)
		}
		indices = append(indices, value)
	}

	return indices, nil
}

func bufferedAccessorRange(document glbDocument, buffers [][]byte, accessor glbAccessor, accessorIndex int, source string) (glbBufferView, []byte, int, error) {
	if accessor.BufferView == nil {
		return glbBufferView{}, nil, 0, fmt.Errorf("accessor %d has no bufferView", accessorIndex)
	}

	view, err := glbBufferViewAt(document, *accessor.BufferView)
	if err != nil {
		return glbBufferView{}, nil, 0, err
	}
	if view.Buffer < 0 || view.Buffer >= len(buffers) {
		return glbBufferView{}, nil, 0, fmt.Errorf("%s accessor %d references unsupported buffer %d", source, accessorIndex, view.Buffer)
	}

	buffer := buffers[view.Buffer]
	start := view.ByteOffset + accessor.ByteOffset
	viewEnd := view.ByteOffset + view.ByteLength
	if start < 0 || viewEnd > len(buffer) || start > viewEnd {
		return glbBufferView{}, nil, 0, fmt.Errorf("accessor %d has invalid buffer range", accessorIndex)
	}
	return view, buffer, start, nil
}
