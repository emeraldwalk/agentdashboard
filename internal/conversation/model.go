package conversation

import "time"

type Status string

const (
	StatusRunning Status = "running"
	StatusWaiting Status = "waiting_input"
	StatusStopped Status = "stopped"
	StatusFailed  Status = "failed"
)

type Source string

const (
	SourceHost   Source = "host"
	SourceDocker Source = "docker"
)

type Conversation struct {
	ID          string    `json:"id"`
	Project     string    `json:"project"`
	Title       string    `json:"title"`
	Status      Status    `json:"status"`
	Source      Source    `json:"source"`
	StartedAt   time.Time `json:"startedAt"`
	LastEventAt time.Time `json:"lastEventAt"`
}
