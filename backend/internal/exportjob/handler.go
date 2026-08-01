package exportjob

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"taverna/backend/internal/assembly"
)

type Handler struct {
	repo        *Repository
	assembly    *assembly.Service
	artifactDir string
	logger      *slog.Logger
}

func NewHandler(repo *Repository, assemblyService *assembly.Service, artifactDir string, logger *slog.Logger) *Handler {
	return &Handler{repo: repo, assembly: assemblyService, artifactDir: artifactDir, logger: logger}
}

func (h *Handler) ProjectExports(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/projects/"), "/exports")
	projectID = strings.Trim(projectID, "/")
	if projectID == "" || strings.Contains(projectID, "/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.createProjectExport(w, r, projectID)
	case http.MethodGet:
		h.listProjectExports(w, r, projectID)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) createProjectExport(w http.ResponseWriter, r *http.Request, projectID string) {
	defer r.Body.Close()
	var request CreateRequest
	if r.Body != nil {
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				request = CreateRequest{}
			} else {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid export payload"})
				return
			}
		}
	}

	if request.Format == "" && request.Target == "" {
		request = CreateRequest{}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	manifest, err := h.assembly.Manifest(ctx, projectID)
	if err != nil {
		switch {
		case errors.Is(err, assembly.ErrProjectNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		case errors.Is(err, assembly.ErrInvalidAssembly):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			h.serverError(w, "build assembly manifest for export", err)
		}
		return
	}

	job, err := h.repo.Create(ctx, projectID, request, manifest)
	if err != nil {
		if errors.Is(err, ErrInvalidRequest) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid export payload"})
			return
		}
		if errors.Is(err, ErrProjectNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
			return
		}
		h.serverError(w, "create export job", err)
		return
	}

	writeJSON(w, http.StatusCreated, job)
}

func (h *Handler) listProjectExports(w http.ResponseWriter, r *http.Request, projectID string) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	jobs, err := h.repo.ListByProject(ctx, projectID)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
			return
		}
		h.serverError(w, "list export jobs", err)
		return
	}

	writeJSON(w, http.StatusOK, jobs)
}

func (h *Handler) Item(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/exports/")
	path = strings.Trim(path, "/")
	if path == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "export job not found"})
		return
	}

	if strings.HasSuffix(path, "/artifact") {
		id := strings.TrimSuffix(path, "/artifact")
		id = strings.Trim(id, "/")
		h.artifact(w, r, id)
		return
	}

	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	id := strings.TrimSpace(path)
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "export job not found"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	job, err := h.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "export job not found"})
			return
		}
		h.serverError(w, "get export job", err)
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (h *Handler) artifact(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "export artifact not found"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	job, err := h.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "export job not found"})
			return
		}
		h.serverError(w, "get export job artifact", err)
		return
	}

	var payload struct {
		ArtifactFile ArtifactResult `json:"artifact_file"`
	}
	if err := json.Unmarshal(job.Artifact, &payload); err != nil || payload.ArtifactFile.StoragePath == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "export artifact file not found"})
		return
	}

	storagePath := filepath.Clean(payload.ArtifactFile.StoragePath)
	storageRoot := filepath.Clean(h.artifactDir)
	rel, err := filepath.Rel(storageRoot, storagePath)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		h.serverError(w, "resolve export artifact", errors.New("artifact path escapes storage dir"))
		return
	}

	if payload.ArtifactFile.ContentType != "" {
		w.Header().Set("Content-Type", payload.ArtifactFile.ContentType)
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+payload.ArtifactFile.FileName+"\"")
	http.ServeFile(w, r, storagePath)
}

func (h *Handler) serverError(w http.ResponseWriter, operation string, err error) {
	h.logger.Error(operation+" failed", "error", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
