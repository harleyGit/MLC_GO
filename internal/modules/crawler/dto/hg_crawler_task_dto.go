package dto

import (
	"encoding/json"
	"time"
)

// HGTaskDefinitionSaveRequest carries editable crawler task fields and the expected optimistic version.
type HGTaskDefinitionSaveRequest struct {
	ID            uint64          `json:"id,omitempty"`
	Name          string          `json:"name"`
	Platform      string          `json:"platform"`
	Enabled       bool            `json:"enabled"`
	Cron          string          `json:"cron"`
	ParserType    string          `json:"parserType"`
	ItemPath      string          `json:"itemPath"`
	MaxItems      uint32          `json:"maxItems"`
	Configuration json.RawMessage `json:"configuration"`
	Version       uint64          `json:"version,omitempty"`
}

// HGTaskDefinitionListRequest uses an increasing database ID cursor and a bounded page size.
type HGTaskDefinitionListRequest struct {
	Cursor uint64 `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// HGTaskRunCreateRequest starts a run for one definition using an immutable configuration snapshot.
type HGTaskRunCreateRequest struct {
	TaskDefinitionID uint64          `json:"taskDefinitionId"`
	Configuration    json.RawMessage `json:"configuration"`
	StartedAt        time.Time       `json:"startedAt,omitempty"`
}

// HGTaskRunCompleteRequest carries the terminal execution summary persisted atomically with the definition summary.
type HGTaskRunCompleteRequest struct {
	RunID            uint64    `json:"runId"`
	TaskDefinitionID uint64    `json:"taskDefinitionId"`
	Status           string    `json:"status"`
	ItemCount        uint32    `json:"itemCount"`
	ErrorMessage     string    `json:"errorMessage,omitempty"`
	StartedAt        time.Time `json:"startedAt"`
	FinishedAt       time.Time `json:"finishedAt,omitempty"`
}
