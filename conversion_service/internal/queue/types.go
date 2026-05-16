package queue

import "time"

type VideoBuildRequested struct {
	JobID          int64     `json:"job_id"`
	PublicationID  int64     `json:"publication_id"`
	Quality        string    `json:"quality"`
	Manifest       string    `json:"manifest"`
	ObjectKey      string    `json:"object_key"`
	RequestedAt    time.Time `json:"requested_at"`
	Attempt        int       `json:"attempt"`
	IdempotencyKey string    `json:"idempotency_key"`
	RequesterToken string    `json:"requester_token"`
}
