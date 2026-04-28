package session

import "time"

type Status string

const (
	StatusRunning Status = "running"
	StatusWaiting Status = "waiting_input"
	StatusStopped Status = "stopped"
	StatusFailed  Status = "failed"
)

type Session struct {
	ID          string
	AgentName   string
	Status      Status
	StartedAt   time.Time
	LastEventAt time.Time
}

type RawEvent struct {
	ID         int64
	Signal     string
	ReceivedAt time.Time
	Payload    string
}
