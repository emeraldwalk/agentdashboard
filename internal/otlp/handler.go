package otlp

import (
	"encoding/json"
	"io"
	"net/http"

	logcollpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	metriccollpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracecollpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/emeraldwalk/agentdashboard/internal/session"
)

const maxBodyBytes = 32 << 20 // 32 MB

// BrokerPublisher is satisfied by dashboard.Broker (plan 04). Using an
// interface here avoids a hard dependency on the dashboard package before it
// exists.
type BrokerPublisher interface {
	Publish([]byte)
}

// RawStore is a narrow interface for persisting raw OTLP payloads.
type RawStore interface {
	AppendRawEvent(signal, payload string) error
}

// Handler handles OTLP HTTP requests.
type Handler struct {
	store  session.Store
	raw    RawStore
	broker BrokerPublisher
}

// NewHandler constructs a Handler with the given store, raw store, and broker.
func NewHandler(store session.Store, raw RawStore, broker BrokerPublisher) *Handler {
	return &Handler{store: store, raw: raw, broker: broker}
}

// RegisterRoutes registers the three OTLP endpoints on mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/traces", h.handleTraces)
	mux.HandleFunc("POST /v1/metrics", h.handleMetrics)
	mux.HandleFunc("POST /v1/logs", h.handleLogs)
}

// readAndCheck reads the request body (capped at maxBodyBytes) and verifies
// that Content-Type is application/x-protobuf. Returns the body bytes, or
// writes an error response and returns nil.
func readAndCheck(w http.ResponseWriter, r *http.Request) []byte {
	if r.Header.Get("Content-Type") != "application/x-protobuf" {
		http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return nil
	}
	return body
}

// captureRaw JSON-encodes a proto message and appends it to the raw store.
func (h *Handler) captureRaw(signal string, msg proto.Message) {
	b, err := protojson.Marshal(msg)
	if err != nil {
		return
	}
	h.raw.AppendRawEvent(signal, string(b)) //nolint:errcheck
}

// upsertAndPublish calls store.Upsert and broker.Publish for each session.
func (h *Handler) upsertAndPublish(sessions []session.Session) {
	for _, sess := range sessions {
		if err := h.store.Upsert(sess); err != nil {
			continue
		}
		data, err := json.Marshal(sess)
		if err != nil {
			continue
		}
		h.broker.Publish(data)
	}
}

func (h *Handler) handleTraces(w http.ResponseWriter, r *http.Request) {
	body := readAndCheck(w, r)
	if body == nil {
		return
	}

	var req tracecollpb.ExportTraceServiceRequest
	if err := proto.Unmarshal(body, &req); err != nil {
		http.Error(w, "failed to parse traces", http.StatusBadRequest)
		return
	}
	h.captureRaw("traces", &req)

	sessions, err := ParseTraces(body)
	if err != nil {
		http.Error(w, "failed to parse traces", http.StatusBadRequest)
		return
	}

	h.upsertAndPublish(sessions)

	resp := &tracecollpb.ExportTraceServiceResponse{}
	out, _ := proto.Marshal(resp)
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	w.Write(out) //nolint:errcheck
}

func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	body := readAndCheck(w, r)
	if body == nil {
		return
	}

	var req metriccollpb.ExportMetricsServiceRequest
	if err := proto.Unmarshal(body, &req); err != nil {
		http.Error(w, "failed to parse metrics", http.StatusBadRequest)
		return
	}
	h.captureRaw("metrics", &req)

	sessions, err := ParseMetrics(body)
	if err != nil {
		http.Error(w, "failed to parse metrics", http.StatusBadRequest)
		return
	}

	h.upsertAndPublish(sessions)

	resp := &metriccollpb.ExportMetricsServiceResponse{}
	out, _ := proto.Marshal(resp)
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	w.Write(out) //nolint:errcheck
}

func (h *Handler) handleLogs(w http.ResponseWriter, r *http.Request) {
	body := readAndCheck(w, r)
	if body == nil {
		return
	}

	var req logcollpb.ExportLogsServiceRequest
	if err := proto.Unmarshal(body, &req); err != nil {
		http.Error(w, "failed to parse logs", http.StatusBadRequest)
		return
	}
	h.captureRaw("logs", &req)

	sessions, err := ParseLogs(body)
	if err != nil {
		http.Error(w, "failed to parse logs", http.StatusBadRequest)
		return
	}

	h.upsertAndPublish(sessions)

	resp := &logcollpb.ExportLogsServiceResponse{}
	out, _ := proto.Marshal(resp)
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	w.Write(out) //nolint:errcheck
}
