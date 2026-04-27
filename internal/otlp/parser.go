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

// ParseTraces decodes a raw protobuf body into a list of Sessions.
// session.id is read from span attributes; service.name from resource attributes.
func ParseTraces(body []byte) ([]session.Session, error) {
	req := &tracepb.ExportTraceServiceRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	// Group spans by session.id so one export batch → one session upsert.
	type spanGroup struct {
		agentName string
		spans     []*tracev1.Span
	}
	groups := map[string]*spanGroup{}

	for _, rs := range req.GetResourceSpans() {
		var resAttrs []*commonpb.KeyValue
		if rs.GetResource() != nil {
			resAttrs = rs.GetResource().GetAttributes()
		}
		agentName := attrValue(resAttrs, "service.name")
		if agentName == "" {
			agentName = "unknown"
		}

		for _, ss := range rs.GetScopeSpans() {
			for _, sp := range ss.GetSpans() {
				// session.id lives on span attributes for Claude Code.
				id := attrValue(sp.GetAttributes(), "session.id")
				if id == "" {
					id = attrValue(resAttrs, "session.id")
				}
				if id == "" {
					id = attrValue(resAttrs, "service.instance.id")
				}
				if id == "" {
					continue
				}
				if groups[id] == nil {
					groups[id] = &spanGroup{agentName: agentName}
				}
				groups[id].spans = append(groups[id].spans, sp)
			}
		}
	}

	var sessions []session.Session
	for id, g := range groups {
		status := deriveTraceStatus(g.spans)

		var startedAt time.Time
		for _, sp := range g.spans {
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
			AgentName:   g.agentName,
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

// ParseMetrics decodes metrics payload. Claude Code does not include session.id
// in metric resource or data-point attributes, so this currently returns nothing.
func ParseMetrics(body []byte) ([]session.Session, error) {
	req := &metricpb.ExportMetricsServiceRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, err
	}
	return nil, nil
}

// ParseLogs decodes logs payload into Sessions.
// session.id is read from individual log record attributes; service.name from resource attributes.
func ParseLogs(body []byte) ([]session.Session, error) {
	req := &logpb.ExportLogsServiceRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	// Deduplicate: one upsert per unique session.id per export batch.
	seen := map[string]bool{}
	var sessions []session.Session

	for _, rl := range req.GetResourceLogs() {
		var resAttrs []*commonpb.KeyValue
		if rl.GetResource() != nil {
			resAttrs = rl.GetResource().GetAttributes()
		}
		agentName := attrValue(resAttrs, "service.name")
		if agentName == "" {
			agentName = "unknown"
		}

		for _, sl := range rl.GetScopeLogs() {
			for _, lr := range sl.GetLogRecords() {
				id := attrValue(lr.GetAttributes(), "session.id")
				if id == "" {
					continue
				}
				if seen[id] {
					continue
				}
				seen[id] = true
				sessions = append(sessions, session.Session{
					ID:          id,
					AgentName:   agentName,
					Status:      session.StatusRunning,
					StartedAt:   now,
					LastEventAt: now,
				})
			}
		}
	}

	return sessions, nil
}
