package otlp

import (
	"strings"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	metricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/emeraldwalk/agentdashboard/internal/session"
)

// attrValue returns the string value of the named attribute from the list,
// or "" if not found.
func attrValue(attrs []*commonpb.KeyValue, name string) string {
	for _, kv := range attrs {
		if kv.GetKey() == name {
			return kv.GetValue().GetStringValue()
		}
	}
	return ""
}

// extractIdentity derives the session ID and agent name from a resource's
// attributes. Returns ("", "") if the resource should be skipped.
func extractIdentity(attrs []*commonpb.KeyValue) (id, agentName string) {
	id = attrValue(attrs, "session.id")
	if id == "" {
		id = attrValue(attrs, "service.instance.id")
	}
	if id == "" {
		return "", ""
	}
	agentName = attrValue(attrs, "service.name")
	if agentName == "" {
		agentName = "unknown"
	}
	return id, agentName
}

// ParseTraces decodes a raw protobuf body into a list of Sessions.
func ParseTraces(body []byte) ([]session.Session, error) {
	req := &tracepb.ExportTraceServiceRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var sessions []session.Session

	for _, rs := range req.GetResourceSpans() {
		res := rs.GetResource()
		var attrs []*commonpb.KeyValue
		if res != nil {
			attrs = res.GetAttributes()
		}

		id, agentName := extractIdentity(attrs)
		if id == "" {
			continue
		}

		// Collect all spans across all scopes.
		var allSpans []*tracev1.Span
		for _, ss := range rs.GetScopeSpans() {
			allSpans = append(allSpans, ss.GetSpans()...)
		}

		// Derive status.
		status := deriveTraceStatus(allSpans)

		// Earliest start time.
		var startedAt time.Time
		for _, sp := range allSpans {
			if sp.GetStartTimeUnixNano() == 0 {
				continue
			}
			t := time.Unix(0, int64(sp.GetStartTimeUnixNano())).UTC()
			if startedAt.IsZero() || t.Before(startedAt) {
				startedAt = t
			}
		}
		if startedAt.IsZero() {
			startedAt = now
		}

		sessions = append(sessions, session.Session{
			ID:          id,
			AgentName:   agentName,
			Status:      status,
			StartedAt:   startedAt,
			LastEventAt: now,
		})
	}

	return sessions, nil
}

// deriveTraceStatus determines the session status from a set of spans.
func deriveTraceStatus(spans []*tracev1.Span) session.Status {
	for _, sp := range spans {
		if sp.GetStatus() != nil && sp.GetStatus().GetCode() == tracev1.Status_STATUS_CODE_ERROR {
			return session.StatusFailed
		}
	}
	for _, sp := range spans {
		name := strings.ToLower(sp.GetName())
		if strings.Contains(name, "waiting") || strings.Contains(name, "user_input") {
			return session.StatusWaiting
		}
	}
	for _, sp := range spans {
		if sp.GetEndTimeUnixNano() == 0 {
			return session.StatusRunning
		}
	}
	return session.StatusStopped
}

// ParseMetrics decodes metrics payload; returns sessions with status "running" as a heartbeat.
func ParseMetrics(body []byte) ([]session.Session, error) {
	req := &metricpb.ExportMetricsServiceRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var sessions []session.Session

	for _, rm := range req.GetResourceMetrics() {
		res := rm.GetResource()
		var attrs []*commonpb.KeyValue
		if res != nil {
			attrs = res.GetAttributes()
		}

		id, agentName := extractIdentity(attrs)
		if id == "" {
			continue
		}

		sessions = append(sessions, session.Session{
			ID:          id,
			AgentName:   agentName,
			Status:      session.StatusRunning,
			StartedAt:   now,
			LastEventAt: now,
		})
	}

	return sessions, nil
}

// ParseLogs decodes logs payload; returns sessions inferred from log records.
func ParseLogs(body []byte) ([]session.Session, error) {
	req := &logpb.ExportLogsServiceRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var sessions []session.Session

	for _, rl := range req.GetResourceLogs() {
		res := rl.GetResource()
		var attrs []*commonpb.KeyValue
		if res != nil {
			attrs = res.GetAttributes()
		}

		id, agentName := extractIdentity(attrs)
		if id == "" {
			continue
		}

		sessions = append(sessions, session.Session{
			ID:          id,
			AgentName:   agentName,
			Status:      session.StatusRunning,
			StartedAt:   now,
			LastEventAt: now,
		})
	}

	return sessions, nil
}
