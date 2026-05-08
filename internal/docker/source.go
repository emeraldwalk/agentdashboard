package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/emeraldwalk/agentdashboard/internal/conversation"
	"github.com/emeraldwalk/agentdashboard/internal/jsonl"
	"github.com/emeraldwalk/agentdashboard/internal/watcher"
)

// Source discovers and tails Claude JSONL logs from Docker containers.
type Source struct {
	socketPath      string
	handler         watcher.Handler
	client          *http.Client
	mu              sync.Mutex
	ingestedVolumes map[string]struct{} // stopped-container volumes already fully read
}

// New creates a Source that reads from the Docker socket.
func New(socketPath string, handler watcher.Handler) *Source {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		},
	}
	return &Source{
		socketPath:      socketPath,
		handler:         handler,
		client:          client,
		ingestedVolumes: make(map[string]struct{}),
	}
}

// Run polls running containers every 10 seconds and ingests stopped containers
// once at startup in a background goroutine so slow stopped-container processing
// does not block the live-refresh loop.
func (s *Source) Run(ctx context.Context) error {
	go s.ingestStopped(ctx)

	s.pollRunning(ctx)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			s.pollRunning(ctx)
		}
	}
}

// pollRunning re-reads all currently running containers on every tick.
func (s *Source) pollRunning(ctx context.Context) {
	volumes, err := s.listClaudeVolumes(ctx)
	if err != nil {
		log.Printf("docker: list volumes: %v", err)
		return
	}
	for _, vol := range volumes {
		project := strings.TrimPrefix(vol, "claude-code-config-")
		containerID, running, err := s.findContainerForVolume(ctx, vol)
		if err != nil {
			log.Printf("docker: find container for %s: %v", vol, err)
			continue
		}
		if running {
			s.tailFilesInContainer(ctx, containerID, project, "/home/vscode/.claude/projects")
		}
	}
}

// ingestStopped processes stopped-container volumes once, skipping any already ingested.
func (s *Source) ingestStopped(ctx context.Context) {
	volumes, err := s.listClaudeVolumes(ctx)
	if err != nil {
		log.Printf("docker: ingestStopped list volumes: %v", err)
		return
	}
	for _, vol := range volumes {
		select {
		case <-ctx.Done():
			return
		default:
		}
		s.mu.Lock()
		_, done := s.ingestedVolumes[vol]
		s.mu.Unlock()
		if done {
			continue
		}

		project := strings.TrimPrefix(vol, "claude-code-config-")
		containerID, running, err := s.findContainerForVolume(ctx, vol)
		if err != nil {
			log.Printf("docker: find container for %s: %v", vol, err)
			continue
		}
		log.Printf("docker: volume=%s containerID=%s running=%v", vol, containerID, running)
		if !running {
			s.readFilesViaTemporaryContainer(ctx, vol, project)
			s.mu.Lock()
			s.ingestedVolumes[vol] = struct{}{}
			s.mu.Unlock()
		}
	}
}

func (s *Source) listClaudeVolumes(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://docker/volumes", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Volumes []struct {
			Name string `json:"Name"`
		} `json:"Volumes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var names []string
	for _, v := range result.Volumes {
		if strings.HasPrefix(v.Name, "claude-code-config-") {
			names = append(names, v.Name)
		}
	}
	return names, nil
}

// findContainerForVolume returns the container ID and whether it is running.
// Queries all containers (running and stopped) via ?all=1.
// Returns an empty containerID if no container has the volume mounted.
func (s *Source) findContainerForVolume(ctx context.Context, volumeName string) (containerID string, running bool, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://docker/containers/json?all=1", nil)
	if err != nil {
		return "", false, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	var containers []struct {
		ID     string `json:"Id"`
		State  string `json:"State"`
		Mounts []struct {
			Name string `json:"Name"`
			Dest string `json:"Destination"`
		} `json:"Mounts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return "", false, err
	}

	for _, c := range containers {
		for _, m := range c.Mounts {
			if m.Name == volumeName && m.Dest == "/home/vscode/.claude" {
				return c.ID, c.State == "running", nil
			}
		}
	}
	return "", false, nil
}

func (s *Source) readFilesViaTemporaryContainer(ctx context.Context, volumeName, project string) {
	createBody, _ := json.Marshal(map[string]any{
		"Image": "alpine",
		"Cmd":   []string{"sleep", "30"},
		"HostConfig": map[string]any{
			"Binds":      []string{volumeName + ":/data"},
			"AutoRemove": true,
		},
	})
	createReq, err := http.NewRequestWithContext(ctx, "POST", "http://docker/containers/create",
		bytes.NewReader(createBody))
	if err != nil {
		log.Printf("docker: create temp container for %s: %v", volumeName, err)
		return
	}
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := s.client.Do(createReq)
	if err != nil {
		log.Printf("docker: create temp container for %s: %v", volumeName, err)
		return
	}
	defer createResp.Body.Close()

	var created struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil || created.ID == "" {
		log.Printf("docker: decode create response for %s: %v", volumeName, err)
		return
	}

	startReq, err := http.NewRequestWithContext(ctx, "POST",
		"http://docker/containers/"+created.ID+"/start",
		bytes.NewReader([]byte("{}")))
	if err != nil {
		log.Printf("docker: start temp container for %s: %v", volumeName, err)
		return
	}
	startReq.Header.Set("Content-Type", "application/json")
	if _, err := s.client.Do(startReq); err != nil {
		log.Printf("docker: start temp container for %s: %v", volumeName, err)
		return
	}

	s.tailFilesInContainer(ctx, created.ID, project, "/data/projects")
}

func (s *Source) tailFilesInContainer(ctx context.Context, containerID, project, findRoot string) {
	output, err := s.execInContainer(ctx, containerID, []string{
		"find", findRoot, "-name", "*.jsonl",
	})
	if err != nil {
		log.Printf("docker: find files in %s: %v", containerID[:12], err)
		return
	}

	paths := strings.Split(strings.TrimSpace(output), "\n")
	log.Printf("docker: %s found %d jsonl files", project, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		s.processContainerFile(ctx, containerID, path, project)
	}
}

func (s *Source) processContainerFile(ctx context.Context, containerID, path, project string) {
	content, err := s.execInContainer(ctx, containerID, []string{"cat", path})
	if err != nil {
		log.Printf("docker: cat %s in %s: %v", path, containerID[:12], err)
		return
	}

	records, err := jsonl.Parse(strings.NewReader(content))
	if err != nil || len(records) == 0 {
		log.Printf("docker: parse %s: err=%v records=%d", path, err, len(records))
		return
	}

	parts := strings.Split(path, "/")
	sessionID := strings.TrimSuffix(parts[len(parts)-1], ".jsonl")

	var title string
	for _, r := range records {
		if r.Type == "ai-title" && r.AITitle != "" {
			title = r.AITitle
		}
	}

	isSubagent := isSubagentPath(path) || jsonl.IsSubagentRecords(sessionID, records)
	parentID := parentIDFromPath(path)
	if parentID == "" {
		parentID = jsonl.ParentIDFromRecords(records)
	}

	startedAt := records[0].Timestamp
	lastEventAt := jsonl.DeriveLastEventAt(records)
	if lastEventAt.IsZero() {
		lastEventAt = startedAt
	}

	c := conversation.Conversation{
		ID:          sessionID,
		Project:     project,
		Title:       title,
		Status:      jsonl.DeriveStatus(records),
		Source:      conversation.SourceDocker,
		StartedAt:   startedAt,
		LastEventAt: lastEventAt,
		IsSubagent:  isSubagent,
		ParentID:    parentID,
	}
	s.handler.OnConversation(c)
}

// isSubagentPath reports whether path is a sub-agent JSONL file.
func isSubagentPath(path string) bool {
	return strings.Contains(path, "/subagents/")
}

// parentIDFromPath extracts the parent session UUID from a subagent path.
// Path form: .../projects/<project>/<parent-uuid>/subagents/<agent-id>.jsonl
// Returns "" if the path is not a subagent path.
func parentIDFromPath(path string) string {
	idx := strings.Index(path, "/subagents/")
	if idx == -1 {
		return ""
	}
	before := path[:idx]
	parts := strings.Split(before, "/")
	return parts[len(parts)-1]
}

func (s *Source) execInContainer(ctx context.Context, containerID string, cmd []string) (string, error) {
	createBody, _ := json.Marshal(map[string]any{
		"AttachStdout": true,
		"AttachStderr": false,
		"Cmd":          cmd,
	})
	createReq, err := http.NewRequestWithContext(ctx, "POST",
		"http://docker/containers/"+containerID+"/exec",
		bytes.NewReader(createBody),
	)
	if err != nil {
		return "", err
	}
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := s.client.Do(createReq)
	if err != nil {
		return "", err
	}
	defer createResp.Body.Close()

	var execID struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&execID); err != nil {
		return "", err
	}

	startBody, _ := json.Marshal(map[string]any{"Detach": false, "Tty": false})
	startReq, err := http.NewRequestWithContext(ctx, "POST",
		"http://docker/exec/"+execID.ID+"/start",
		bytes.NewReader(startBody),
	)
	if err != nil {
		return "", err
	}
	startReq.Header.Set("Content-Type", "application/json")

	startResp, err := s.client.Do(startReq)
	if err != nil {
		return "", err
	}
	defer startResp.Body.Close()

	return stripDockerMux(startResp.Body)
}

// stripDockerMux reads Docker's multiplexed stream format and returns stdout as a string.
// Each frame has an 8-byte header: header[0]=stream type, header[4-7]=uint32 payload size.
func stripDockerMux(r io.Reader) (string, error) {
	var sb strings.Builder
	header := make([]byte, 8)
	for {
		_, err := io.ReadFull(r, header)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return sb.String(), err
		}
		size := uint32(header[4])<<24 | uint32(header[5])<<16 | uint32(header[6])<<8 | uint32(header[7])
		frame := make([]byte, size)
		if _, err := io.ReadFull(r, frame); err != nil {
			break
		}
		if header[0] == 1 { // stdout only
			sb.Write(frame)
		}
	}
	return sb.String(), nil
}
