package modelasset

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func Read3MFTriangles(path string) ([]Triangle, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("read 3MF archive: %w", err)
	}
	defer reader.Close()

	modelFile := find3MFModelFile(reader.File)
	if modelFile == nil {
		return nil, fmt.Errorf("3MF model file is missing")
	}

	source, err := modelFile.Open()
	if err != nil {
		return nil, fmt.Errorf("open 3MF model file: %w", err)
	}
	defer source.Close()

	triangles, err := parse3MFModelTriangles(source)
	if err != nil {
		return nil, err
	}
	if len(triangles) == 0 {
		return nil, fmt.Errorf("3MF contains no mesh triangles")
	}
	return triangles, nil
}

func inspect3MFGeometry(path string) (GeometryMetadata, error) {
	triangles, err := Read3MFTriangles(path)
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
	return finishGeometryMetadata("3mf", vertexCount, triangleCount, triangleCount, bounds, []string{}), nil
}

func find3MFModelFile(files []*zip.File) *zip.File {
	for _, file := range files {
		if strings.EqualFold(file.Name, "3D/3dmodel.model") {
			return file
		}
	}
	for _, file := range files {
		if strings.HasSuffix(strings.ToLower(file.Name), ".model") {
			return file
		}
	}
	return nil
}

func parse3MFModelTriangles(source io.Reader) ([]Triangle, error) {
	decoder := xml.NewDecoder(source)
	currentVertices := []Vector3{}
	triangles := []Triangle{}
	inVertices := false
	inTriangles := false

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("parse 3MF model XML: %w", err)
		}

		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "vertices":
				currentVertices = []Vector3{}
				inVertices = true
			case "vertex":
				if !inVertices {
					continue
				}
				vertex, err := parse3MFVertex(element)
				if err != nil {
					return nil, err
				}
				currentVertices = append(currentVertices, vertex)
			case "triangles":
				inTriangles = true
			case "triangle":
				if !inTriangles {
					continue
				}
				triangle, err := parse3MFTriangle(element, currentVertices)
				if err != nil {
					return nil, err
				}
				triangles = append(triangles, triangle)
			}
		case xml.EndElement:
			switch element.Name.Local {
			case "vertices":
				inVertices = false
			case "triangles":
				inTriangles = false
			}
		}
	}

	return triangles, nil
}

func parse3MFVertex(element xml.StartElement) (Vector3, error) {
	x, err := parse3MFVectorAttr(element, "x")
	if err != nil {
		return Vector3{}, err
	}
	y, err := parse3MFVectorAttr(element, "y")
	if err != nil {
		return Vector3{}, err
	}
	z, err := parse3MFVectorAttr(element, "z")
	if err != nil {
		return Vector3{}, err
	}
	return Vector3{x, y, z}, nil
}

func parse3MFTriangle(element xml.StartElement, vertices []Vector3) (Triangle, error) {
	v1, err := parse3MFIndexAttr(element, "v1")
	if err != nil {
		return Triangle{}, err
	}
	v2, err := parse3MFIndexAttr(element, "v2")
	if err != nil {
		return Triangle{}, err
	}
	v3, err := parse3MFIndexAttr(element, "v3")
	if err != nil {
		return Triangle{}, err
	}

	for _, index := range []int{v1, v2, v3} {
		if index < 0 || index >= len(vertices) {
			return Triangle{}, fmt.Errorf("3MF triangle references vertex %d outside count %d", index, len(vertices))
		}
	}
	return Triangle{vertices[v1], vertices[v2], vertices[v3]}, nil
}

func parse3MFVectorAttr(element xml.StartElement, name string) (float64, error) {
	value, ok := localAttrValue(element, name)
	if !ok {
		return 0, fmt.Errorf("3MF vertex is missing %s", name)
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("3MF vertex has invalid %s: %w", name, err)
	}
	return parsed, nil
}

func parse3MFIndexAttr(element xml.StartElement, name string) (int, error) {
	value, ok := localAttrValue(element, name)
	if !ok {
		return 0, fmt.Errorf("3MF triangle is missing %s", name)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("3MF triangle has invalid %s: %w", name, err)
	}
	return parsed, nil
}

func localAttrValue(element xml.StartElement, name string) (string, bool) {
	for _, attr := range element.Attr {
		if attr.Name.Local == name {
			return attr.Value, true
		}
	}
	return "", false
}
