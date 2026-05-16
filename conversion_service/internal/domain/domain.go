package domain

import "time"

type ExpiredAsset struct {
	PublicacionID int64
	Calidad       string
	Manifiesto    string
}

type JobStatus string

const (
	JobQueued     JobStatus = "queued"
	JobProcessing JobStatus = "processing"
	JobCompleted  JobStatus = "completed"
	JobFailed     JobStatus = "failed"
	JobExpired    JobStatus = "expired"
)

func (s JobStatus) IsActive() bool {
	return s == JobQueued || s == JobProcessing
}

func (s JobStatus) IsTerminal() bool {
	return s == JobCompleted || s == JobFailed || s == JobExpired
}

type Job struct {
	ID             int64
	PublicacionID  int64
	Calidad        string
	Estado         JobStatus
	Error          string
	RequesterToken string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	FinishedAt     *time.Time
}
