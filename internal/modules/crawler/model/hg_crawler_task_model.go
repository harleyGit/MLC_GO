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

// HGTaskExternalContent is one task association joined with normalized external content for the Ops UI.
type HGTaskExternalContent struct {
	AssociationID        uint64     `json:"associationId"`
	TaskDefinitionID     uint64     `json:"taskDefinitionId"`
	LastRunID            uint64     `json:"lastRunId"`
	ExternalContentID    uint64     `json:"externalContentRowId"`
	Platform             string     `json:"platform"`
	ContentID            string     `json:"contentId"`
	Title                string     `json:"title"`
	AuthorID             string     `json:"authorId"`
	AuthorName           string     `json:"authorName"`
	CoverURL             string     `json:"coverUrl"`
	TargetURL            string     `json:"targetUrl"`
	DurationSeconds      uint32     `json:"durationSeconds"`
	ViewCount            uint64     `json:"viewCount"`
	LikeCount            uint64     `json:"likeCount"`
	CommentCount         uint64     `json:"commentCount"`
	PublishedAt          *time.Time `json:"publishedAt,omitempty"`
	FirstSeenAt          time.Time  `json:"firstSeenAt"`
	LastSeenAt           time.Time  `json:"lastSeenAt"`
	ContentCreatedAt     time.Time  `json:"contentCreatedAt"`
	ContentUpdatedAt     time.Time  `json:"contentUpdatedAt"`
	AssociatedAt         time.Time  `json:"associatedAt"`
	AssociationUpdatedAt time.Time  `json:"associationUpdatedAt"`
}
