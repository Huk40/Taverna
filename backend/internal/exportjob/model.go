package exportjob

import (
	"encoding/json"
	"time"
)

type JobStatus string

const (
	StatusQueued     JobStatus = "queued"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
)

type ExportJob struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"project_id"`
	Target    string          `json:"target"`
	Format    string          `json:"format"`
	Status    JobStatus       `json:"status"`
	Artifact  json.RawMessage `json:"artifact"`
	Error     *string         `json:"error"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type CreateRequest struct {
	Target string `json:"target"`
	Format string `json:"format"`
}
