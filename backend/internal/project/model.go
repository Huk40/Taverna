package project

import (
	"encoding/json"
	"time"
)

type Project struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type ProjectPayload struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}
