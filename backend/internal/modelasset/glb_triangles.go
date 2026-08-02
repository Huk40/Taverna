package modelasset

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

type Triangle [3]Vector3

func ReadGLBTriangles(path string) ([]Triangle, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read GLB triangles: %w", err)
	}

	document, binChunk, _, err := parseGLB(raw)
	if err != nil {
		return nil, err
	}

	triangles := make([]Triangle, 0)
	for meshIndex, mesh := range document.Meshes {
		for primitiveIndex, primitive := range mesh.Primitives {
			positionAccessorIndex, ok := primitive.Attributes["POSITION"]
			if !ok {
				return nil, fmt.Errorf("GLB mesh %d primitive %d has no POSITION attribute", meshIndex, primitiveIndex)
			}

			positions, err := readGLBPositionAccessor(document, binChunk, positionAccessorIndex)
			if err != nil {
				return nil, fmt.Errorf("read GLB mesh %d primitive %d positions: %w", meshIndex, primitiveIndex, err)
			}

			indices, err := readGLBPrimitiveIndices(document, binChunk, primitive, len(positions))
			if err != nil {
				return nil, fmt.Errorf("read GLB mesh %d primitive %d indices: %w", meshIndex, primitiveIndex, err)
			}

			triangles = append(triangles, trianglesForGLBPrimitive(positions, indices, primitiveMode(primitive))...)
		}
	}

	if len(triangles) == 0 {
		return nil, fmt.Errorf("GLB contains no triangle primitives")
	}
	return triangles, nil
}

func readGLBPositionAccessor(document glbDocument, binChunk []byte, accessorIndex int) ([]Vector3, error) {
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

	view, start, err := glbAccessorBufferRange(document, binChunk, accessor, accessorIndex)
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
		if offset+12 > viewEnd || offset+12 > len(binChunk) {
			return nil, fmt.Errorf("POSITION accessor %d reads past bufferView", accessorIndex)
		}
		positions = append(positions, Vector3{
			float64(math.Float32frombits(binary.LittleEndian.Uint32(binChunk[offset : offset+4]))),
			float64(math.Float32frombits(binary.LittleEndian.Uint32(binChunk[offset+4 : offset+8]))),
			float64(math.Float32frombits(binary.LittleEndian.Uint32(binChunk[offset+8 : offset+12]))),
		})
	}

	return positions, nil
}

func readGLBPrimitiveIndices(document glbDocument, binChunk []byte, primitive glbPrimitive, positionCount int) ([]int, error) {
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
	view, start, err := glbAccessorBufferRange(document, binChunk, accessor, accessorIndex)
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
		if offset+componentSize > viewEnd || offset+componentSize > len(binChunk) {
			return nil, fmt.Errorf("index accessor %d reads past bufferView", accessorIndex)
		}
		value := glbIndexValue(binChunk[offset:offset+componentSize], accessor.ComponentType)
		if value < 0 || value >= positionCount {
			return nil, fmt.Errorf("index %d references vertex %d outside POSITION count %d", index, value, positionCount)
		}
		indices = append(indices, value)
	}

	return indices, nil
}

func glbAccessorBufferRange(document glbDocument, binChunk []byte, accessor glbAccessor, accessorIndex int) (glbBufferView, int, error) {
	if accessor.BufferView == nil {
		return glbBufferView{}, 0, fmt.Errorf("accessor %d has no bufferView", accessorIndex)
	}
	if len(binChunk) == 0 {
		return glbBufferView{}, 0, fmt.Errorf("accessor %d requires a BIN chunk", accessorIndex)
	}
	view, err := glbBufferViewAt(document, *accessor.BufferView)
	if err != nil {
		return glbBufferView{}, 0, err
	}
	if view.Buffer != 0 {
		return glbBufferView{}, 0, fmt.Errorf("accessor %d references unsupported buffer %d", accessorIndex, view.Buffer)
	}

	start := view.ByteOffset + accessor.ByteOffset
	viewEnd := view.ByteOffset + view.ByteLength
	if start < 0 || viewEnd > len(binChunk) || start > viewEnd {
		return glbBufferView{}, 0, fmt.Errorf("accessor %d has invalid buffer range", accessorIndex)
	}
	return view, start, nil
}

func glbIndexComponentSize(componentType int) (int, error) {
	switch componentType {
	case 5121:
		return 1, nil
	case 5123:
		return 2, nil
	case 5125:
		return 4, nil
	default:
		return 0, fmt.Errorf("unsupported componentType %d", componentType)
	}
}

func glbIndexValue(raw []byte, componentType int) int {
	switch componentType {
	case 5121:
		return int(raw[0])
	case 5123:
		return int(binary.LittleEndian.Uint16(raw))
	case 5125:
		return int(binary.LittleEndian.Uint32(raw))
	default:
		return 0
	}
}

func trianglesForGLBPrimitive(positions []Vector3, indices []int, mode int) []Triangle {
	triangles := make([]Triangle, 0)
	switch mode {
	case 4:
		for index := 0; index+2 < len(indices); index += 3 {
			triangles = append(triangles, Triangle{positions[indices[index]], positions[indices[index+1]], positions[indices[index+2]]})
		}
	case 5:
		for index := 0; index+2 < len(indices); index++ {
			a, b, c := indices[index], indices[index+1], indices[index+2]
			if index%2 == 1 {
				b, c = c, b
			}
			triangles = append(triangles, Triangle{positions[a], positions[b], positions[c]})
		}
	case 6:
		if len(indices) < 3 {
			return triangles
		}
		for index := 1; index+1 < len(indices); index++ {
			triangles = append(triangles, Triangle{positions[indices[0]], positions[indices[index]], positions[indices[index+1]]})
		}
	}
	return triangles
}

func primitiveMode(primitive glbPrimitive) int {
	if primitive.Mode == nil {
		return 4
	}
	return *primitive.Mode
}
