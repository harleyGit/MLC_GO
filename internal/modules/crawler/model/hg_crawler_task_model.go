package model

import (
	"encoding/json"
	"time"
)

// HGTaskDefinition is the persisted crawler schedule and parser configuration.
type HGTaskDefinition struct {
	ID                uint64          `json:"id"`
	Name              string          `json:"name"`
	Platform          string          `json:"platform"`
	Enabled           bool            `json:"enabled"`
	Cron              string          `json:"cron"`
	ParserType        string          `json:"parserType"`
	ItemPath          string          `json:"itemPath"`
	MaxItems          uint32          `json:"maxItems"`
	Configuration     json.RawMessage `json:"configuration,omitempty"`
	LastRunID         uint64          `json:"lastRunId,omitempty"`
	LastRunStatus     string          `json:"lastRunStatus"`
	LastRunStartedAt  *time.Time      `json:"lastRunStartedAt,omitempty"`
	LastRunFinishedAt *time.Time      `json:"lastRunFinishedAt,omitempty"`
	LastRunItemCount  uint32          `json:"lastRunItemCount"`
	LastRunError      string          `json:"lastRunError,omitempty"`
	Version           uint64          `json:"version"`
	CreatedBy         string          `json:"createdBy"`
	UpdatedBy         string          `json:"updatedBy"`
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
}

// HGTaskRun is one immutable-start crawler execution whose terminal fields are completed once.
type HGTaskRun struct {
	ID               uint64          `json:"id"`
	TaskDefinitionID uint64          `json:"taskDefinitionId"`
	Status           string          `json:"status"`
	Configuration    json.RawMessage `json:"configuration,omitempty"`
	StartedAt        time.Time       `json:"startedAt"`
	FinishedAt       *time.Time      `json:"finishedAt,omitempty"`
	ItemCount        uint32          `json:"itemCount"`
	ErrorMessage     string          `json:"errorMessage,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}
