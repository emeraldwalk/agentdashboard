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
	"time"

	"github.com/emeraldwalk/agentdashboard/internal/conversation"
	"github.com/emeraldwalk/agentdashboard/internal/jsonl"
	"github.com/emeraldwalk/agentdashboard/internal/watcher"
)

// Source discovers and tails Claude JSONL logs from Docker containers.
type Source struct {
	socketPath string
	handler    watcher.Handler
	client     *http.Client
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
	return &Source{socketPath: socketPath, handler: handler, client: client}
}

// Run discovers containers with claude-code-config-* volumes every 30 seconds until ctx is cancelled.
func (s *Source) Run(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	s.discover(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			s.discover(ctx)
		}
	}
}

func (s *Source) discover(ctx context.Context) {
	volumes, err := s.listClaudeVolumes(ctx)
	if err != nil {
		log.Printf("docker: list volumes: %v", err)
		return
	}

	for _, vol := range volumes {
		project := strings.TrimPrefix(vol, "claude-code-config-")
		containerID, err := s.findContainerForVolume(ctx, vol)
		if err != nil || containerID == "" {
			continue
		}
		s.tailFilesInContainer(ctx, containerID, project)
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

func (s *Source) findContainerForVolume(ctx context.Context, volumeName string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://docker/containers/json", nil)
	if err != nil {
		return "", err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var containers []struct {
		ID     string `json:"Id"`
		Mounts []struct {
			Name string `json:"Name"`
			Dest string `json:"Destination"`
		} `json:"Mounts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return "", err
	}

	for _, c := range containers {
		for _, m := range c.Mounts {
			if m.Name == volumeName && m.Dest == "/home/vscode/.claude" {
				return c.ID, nil
			}
		}
	}
	return "", nil
}

func (s *Source) tailFilesInContainer(ctx context.Context, containerID, project string) {
	output, err := s.execInContainer(ctx, containerID, []string{
		"find", "/home/vscode/.claude/projects", "-name", "*.jsonl",
	})
	if err != nil {
		log.Printf("docker: find files in %s: %v", containerID[:12], err)
		return
	}

	for _, path := range strings.Split(strings.TrimSpace(output), "\n") {
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
		StartedAt:   startedAt,
		LastEventAt: lastEventAt,
	}
	s.handler.OnConversation(c)
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
