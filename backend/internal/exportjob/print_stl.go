package exportjob

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"taverna/backend/internal/assembly"
	"taverna/backend/internal/catalog"
)

var ErrPrintGeometry = errors.New("invalid print geometry")

// PrintSTL exports the current selection without saving or modifying a project.
func (h *Handler) PrintSTL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	var payload struct {
		Config catalog.CharacterConfig `json:"config"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Не удалось прочитать выбранную модель."})
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Ожидается одна конфигурация модели."})
		return
	}
	if len(payload.Config.Assembly.Placements) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Сначала выберите модель."})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	manifest, err := h.assembly.ManifestForConfig(ctx, "download", payload.Config)
	if err != nil {
		if errors.Is(err, assembly.ErrInvalidAssembly) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Не удалось собрать выбранные детали. Обновите страницу и выберите модели снова."})
		} else {
			h.serverError(w, "resolve print selection", err)
		}
		return
	}
	data, err := buildPrintSTL(ctx, manifest)
	if err != nil {
		if errors.Is(err, ErrPrintGeometry) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "Не удалось подготовить геометрию выбранных деталей для STL. Проверьте исходные модели."})
		} else {
			h.serverError(w, "build print STL", err)
		}
		return
	}
	w.Header().Set("Content-Type", "model/stl")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"taverna-%dmm.stl\"", manifest.ScaleMM))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

type printTriangle [3][3]float64

// The preview uses Y-up scene units. STL uses millimetres with Z-up and its
// lowest point on the print bed. ScaleMM is the total height including the base.
func buildPrintSTL(ctx context.Context, manifest assembly.Manifest) ([]byte, error) {
	if len(manifest.Placements) == 0 || manifest.ScaleMM < 24 || manifest.ScaleMM > 75 {
		return nil, ErrPrintGeometry
	}
	triangles := make([]printTriangle, 0, 8192)
	min := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	max := [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	collect := func(a, b, c [3]float64) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(triangles) >= 1_000_000 {
			return fmt.Errorf("%w: too many triangles", ErrPrintGeometry)
		}
		triangle := printTriangle{a, b, c}
		for _, point := range triangle {
			for _, value := range point {
				if math.IsNaN(value) || math.IsInf(value, 0) {
					return fmt.Errorf("%w: non-finite vertex", ErrPrintGeometry)
				}
			}
		}
		if triangleNormal(a, b, c) == [3]float64{} {
			return nil
		}
		triangles = append(triangles, triangle)
		for _, point := range triangle {
			for axis, value := range point {
				min[axis] = math.Min(min[axis], value)
				max[axis] = math.Max(max[axis], value)
			}
		}
		return nil
	}
	customBase := false
	for _, placement := range manifest.Placements {
		before := len(triangles)
		_, err := visitSTLPlacement(collect, placement)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("%w: asset %s: %v", ErrPrintGeometry, placement.Asset.ID, err)
		}
		if before == len(triangles) {
			return nil, fmt.Errorf("%w: empty asset %s", ErrPrintGeometry, placement.Asset.ID)
		}
		customBase = customBase || placement.Slot == "base"
	}
	if !customBase {
		scale := manifest.CharacterScale
		if scale <= 0 || math.IsInf(scale, 0) || math.IsNaN(scale) {
			return nil, ErrPrintGeometry
		}
		if err := printBase(collect, 0.76*scale, 0.16*scale); err != nil {
			return nil, err
		}
	}
	height := max[1] - min[1]
	if height <= 1e-9 {
		return nil, ErrPrintGeometry
	}
	factor := float64(manifest.ScaleMM) / height
	centerX, centerZ := (min[0]+max[0])/2, (min[2]+max[2])/2
	data := make([]byte, 84+50*len(triangles))
	copy(data, "Taverna STL | millimetres | Z up | height includes base")
	binary.LittleEndian.PutUint32(data[80:84], uint32(len(triangles)))
	for index, triangle := range triangles {
		if index%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		for vertex, p := range triangle {
			triangle[vertex] = [3]float64{(p[0] - centerX) * factor, -(p[2] - centerZ) * factor, (p[1] - min[1]) * factor}
		}
		normal := triangleNormal(triangle[0], triangle[1], triangle[2])
		offset := 84 + index*50
		for _, point := range [4][3]float64{normal, triangle[0], triangle[1], triangle[2]} {
			for _, value := range point {
				if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > math.MaxFloat32 {
					return nil, ErrPrintGeometry
				}
				binary.LittleEndian.PutUint32(data[offset:offset+4], math.Float32bits(float32(value)))
				offset += 4
			}
		}
	}
	return data, nil
}

func printBase(writeFacet triangleWriter, radius, height float64) error {
	const segments = 64
	for i := 0; i < segments; i++ {
		a, b := float64(i)*2*math.Pi/segments, float64(i+1)*2*math.Pi/segments
		p := [3]float64{radius * math.Cos(a), 0, radius * math.Sin(a)}
		q := [3]float64{radius * math.Cos(b), 0, radius * math.Sin(b)}
		pt, qt := p, q
		pt[1], qt[1] = height, height
		for _, tri := range []printTriangle{{{0, 0, 0}, p, q}, {{0, height, 0}, qt, pt}, {p, pt, q}, {q, pt, qt}} {
			if err := writeFacet(tri[0], tri[1], tri[2]); err != nil {
				return err
			}
		}
	}
	return nil
}
