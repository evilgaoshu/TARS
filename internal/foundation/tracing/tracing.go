// Package tracing provides a lightweight OTLP exporter handle.
//
// When OTLP is enabled (endpoint configured + at least one signal enabled),
// Provider exposes Ping() which performs a TCP connectivity check to verify
// the endpoint is reachable. This gives a real, verifiable exporter path
// without pulling in the full OpenTelemetry SDK dependency tree.
//
// To upgrade to a full SDK-backed exporter, replace Ping() with a proper
// go.opentelemetry.io/otel/exporters/otlp/* initialisation.
package tracing

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"tars/internal/foundation/config"
)

// Provider holds OTLP exporter configuration and provides connectivity checks.
type Provider struct {
	endpoint      string
	protocol      string
	insecure      bool
	metricsEnable bool
	logsEnable    bool
	tracesEnable  bool
}

// New creates a Provider from config.
func New(cfg config.ObservabilityOTLPConfig) Provider {
	protocol := strings.TrimSpace(cfg.Protocol)
	if protocol == "" {
		protocol = "grpc"
	}
	return Provider{
		endpoint:      strings.TrimSpace(cfg.Endpoint),
		protocol:      protocol,
		insecure:      cfg.Insecure,
		metricsEnable: cfg.MetricsEnable,
		logsEnable:    cfg.LogsEnable,
		tracesEnable:  cfg.TracesEnable,
	}
}

// Enabled returns true when a non-empty endpoint is configured with at least one signal.
func (p Provider) Enabled() bool {
	return p.endpoint != "" && (p.metricsEnable || p.logsEnable || p.tracesEnable)
}

// Name returns a human-readable description of the exporter.
func (p Provider) Name() string {
	if !p.Enabled() {
		return "disabled"
	}
	return "otlp/" + p.protocol
}

// Endpoint returns the configured OTLP endpoint.
func (p Provider) Endpoint() string {
	return p.endpoint
}

// EnabledSignals returns the list of enabled signal names (metrics/logs/traces).
func (p Provider) EnabledSignals() []string {
	var signals []string
	if p.metricsEnable {
		signals = append(signals, "metrics")
	}
	if p.logsEnable {
		signals = append(signals, "logs")
	}
	if p.tracesEnable {
		signals = append(signals, "traces")
	}
	return signals
}

// Ping performs a TCP connectivity check to the configured OTLP endpoint.
// Returns nil when the endpoint is reachable or OTLP is disabled.
// This is the minimal verifiable exporter path: confirms network reachability
// before the application attempts to export telemetry.
func (p Provider) Ping(ctx context.Context) error {
	if !p.Enabled() {
		return nil
	}
	host := p.endpoint
	// Strip scheme if present (e.g. "http://host:4317" → "host:4317").
	for _, prefix := range []string{"https://", "http://"} {
		host = strings.TrimPrefix(host, prefix)
	}
	// Default gRPC port.
	if !strings.Contains(host, ":") {
		host = host + ":4317"
	}
	deadline := 5 * time.Second
	if dl, ok := ctx.Deadline(); ok {
		if remaining := time.Until(dl); remaining < deadline {
			deadline = remaining
		}
	}
	conn, err := net.DialTimeout("tcp", host, deadline)
	if err != nil {
		return fmt.Errorf("otlp endpoint unreachable (%s): %w", host, err)
	}
	_ = conn.Close()
	return nil
}
