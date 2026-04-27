package otlp_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logcollpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	metriccollpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracecollpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	logpb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/emeraldwalk/agentdashboard/internal/otlp"
	"github.com/emeraldwalk/agentdashboard/internal/session"
)

// --- stubs ---

type stubStore struct {
	upserted []session.Session
}

func (s *stubStore) Upsert(sess session.Session) error {
	s.upserted = append(s.upserted, sess)
	return nil
}

func (s *stubStore) List() ([]session.Session, error) { return nil, nil }
func (s *stubStore) Close() error                      { return nil }

type stubBroker struct {
	published [][]byte
}

func (b *stubBroker) Publish(data []byte) {
	b.published = append(b.published, data)
}

// --- helpers ---

func resourceWithAttrs(sessionID, serviceName string) *resourcepb.Resource {
	return &resourcepb.Resource{
		Attributes: []*commonpb.KeyValue{
			{Key: "session.id", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: sessionID}}},
			{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: serviceName}}},
		},
	}
}

func postProto(t *testing.T, handler http.Handler, path string, msg proto.Message) *httptest.ResponseRecorder {
	t.Helper()
	body, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-protobuf")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// --- tests ---

func TestHandleTraces_200AndUpsert(t *testing.T) {
	store := &stubStore{}
	broker := &stubBroker{}
	h := otlp.NewHandler(store, broker)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := &tracecollpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: resourceWithAttrs("sess-1", "my-agent"),
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								Name:                 "some-span",
								StartTimeUnixNano:    1_000_000_000,
								EndTimeUnixNano:      2_000_000_000,
							},
						},
					},
				},
			},
		},
	}

	rr := postProto(t, mux, "/v1/traces", req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if len(store.upserted) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(store.upserted))
	}
	sess := store.upserted[0]
	if sess.ID != "sess-1" {
		t.Errorf("expected ID sess-1, got %q", sess.ID)
	}
	if sess.AgentName != "my-agent" {
		t.Errorf("expected AgentName my-agent, got %q", sess.AgentName)
	}
	if sess.Status != session.StatusStopped {
		t.Errorf("expected status stopped, got %q", sess.Status)
	}
	if len(broker.published) != 1 {
		t.Fatalf("expected 1 publish, got %d", len(broker.published))
	}
	// Response body should be a valid (empty) protobuf.
	var respMsg tracecollpb.ExportTraceServiceResponse
	if err := proto.Unmarshal(rr.Body.Bytes(), &respMsg); err != nil {
		t.Errorf("response body is not valid protobuf: %v", err)
	}
}

func TestHandleTraces_ErrorStatus(t *testing.T) {
	store := &stubStore{}
	broker := &stubBroker{}
	h := otlp.NewHandler(store, broker)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := &tracecollpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: resourceWithAttrs("sess-err", "agent"),
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								Name:             "bad-span",
								StartTimeUnixNano: 1_000_000_000,
								EndTimeUnixNano:  2_000_000_000,
								Status: &tracepb.Status{
									Code: tracepb.Status_STATUS_CODE_ERROR,
								},
							},
						},
					},
				},
			},
		},
	}

	rr := postProto(t, mux, "/v1/traces", req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if len(store.upserted) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(store.upserted))
	}
	if store.upserted[0].Status != session.StatusFailed {
		t.Errorf("expected failed, got %q", store.upserted[0].Status)
	}
}

func TestHandleTraces_WaitingStatus(t *testing.T) {
	store := &stubStore{}
	broker := &stubBroker{}
	h := otlp.NewHandler(store, broker)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := &tracecollpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: resourceWithAttrs("sess-wait", "agent"),
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								Name:             "Waiting for user_input",
								StartTimeUnixNano: 1_000_000_000,
								EndTimeUnixNano:  2_000_000_000,
							},
						},
					},
				},
			},
		},
	}

	rr := postProto(t, mux, "/v1/traces", req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if store.upserted[0].Status != session.StatusWaiting {
		t.Errorf("expected waiting, got %q", store.upserted[0].Status)
	}
}

func TestHandleTraces_RunningStatus(t *testing.T) {
	store := &stubStore{}
	broker := &stubBroker{}
	h := otlp.NewHandler(store, broker)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := &tracecollpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: resourceWithAttrs("sess-run", "agent"),
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								Name:             "active-span",
								StartTimeUnixNano: 1_000_000_000,
								EndTimeUnixNano:  0, // still open
							},
						},
					},
				},
			},
		},
	}

	rr := postProto(t, mux, "/v1/traces", req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if store.upserted[0].Status != session.StatusRunning {
		t.Errorf("expected running, got %q", store.upserted[0].Status)
	}
}

func TestHandleMetrics_200AndUpsert(t *testing.T) {
	store := &stubStore{}
	broker := &stubBroker{}
	h := otlp.NewHandler(store, broker)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := &metriccollpb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricpb.ResourceMetrics{
			{
				Resource: resourceWithAttrs("sess-m", "metric-agent"),
			},
		},
	}

	rr := postProto(t, mux, "/v1/metrics", req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if len(store.upserted) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(store.upserted))
	}
	sess := store.upserted[0]
	if sess.ID != "sess-m" {
		t.Errorf("expected ID sess-m, got %q", sess.ID)
	}
	if sess.Status != session.StatusRunning {
		t.Errorf("expected running, got %q", sess.Status)
	}
	var respMsg metriccollpb.ExportMetricsServiceResponse
	if err := proto.Unmarshal(rr.Body.Bytes(), &respMsg); err != nil {
		t.Errorf("response body is not valid protobuf: %v", err)
	}
}

func TestHandleLogs_200AndUpsert(t *testing.T) {
	store := &stubStore{}
	broker := &stubBroker{}
	h := otlp.NewHandler(store, broker)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := &logcollpb.ExportLogsServiceRequest{
		ResourceLogs: []*logpb.ResourceLogs{
			{
				Resource: resourceWithAttrs("sess-l", "log-agent"),
				ScopeLogs: []*logpb.ScopeLogs{
					{
						LogRecords: []*logpb.LogRecord{
							{TimeUnixNano: 1_000_000_000},
						},
					},
				},
			},
		},
	}

	rr := postProto(t, mux, "/v1/logs", req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if len(store.upserted) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(store.upserted))
	}
	sess := store.upserted[0]
	if sess.ID != "sess-l" {
		t.Errorf("expected ID sess-l, got %q", sess.ID)
	}
	if sess.Status != session.StatusRunning {
		t.Errorf("expected running, got %q", sess.Status)
	}
	var respMsg logcollpb.ExportLogsServiceResponse
	if err := proto.Unmarshal(rr.Body.Bytes(), &respMsg); err != nil {
		t.Errorf("response body is not valid protobuf: %v", err)
	}
}

func TestHandleTraces_WrongContentType(t *testing.T) {
	store := &stubStore{}
	broker := &stubBroker{}
	h := otlp.NewHandler(store, broker)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnsupportedMediaType {
		t.Errorf("expected 415, got %d", rr.Code)
	}
	if len(store.upserted) != 0 {
		t.Errorf("expected no upserts, got %d", len(store.upserted))
	}
}

func TestHandleTraces_NoSessionID_Skipped(t *testing.T) {
	store := &stubStore{}
	broker := &stubBroker{}
	h := otlp.NewHandler(store, broker)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Resource with no session.id or service.instance.id.
	req := &tracecollpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "orphan"}}},
					},
				},
			},
		},
	}

	rr := postProto(t, mux, "/v1/traces", req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if len(store.upserted) != 0 {
		t.Errorf("expected 0 upserts (no session id), got %d", len(store.upserted))
	}
}

func TestHandleTraces_FallbackToServiceInstanceID(t *testing.T) {
	store := &stubStore{}
	broker := &stubBroker{}
	h := otlp.NewHandler(store, broker)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := &tracecollpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.instance.id", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "instance-99"}}},
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "my-svc"}}},
					},
				},
			},
		},
	}

	rr := postProto(t, mux, "/v1/traces", req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if len(store.upserted) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(store.upserted))
	}
	if store.upserted[0].ID != "instance-99" {
		t.Errorf("expected ID instance-99, got %q", store.upserted[0].ID)
	}
}
