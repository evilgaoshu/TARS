package metrics

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Labels map[string]string

type sample struct {
	labels Labels
	value  float64
}

type histogramSample struct {
	labels  Labels
	buckets []float64
	counts  []uint64
	sum     float64
	count   uint64
}

type family struct {
	name       string
	help       string
	typ        string
	samples    map[string]*sample
	histograms map[string]*histogramSample
}

type ComponentStatus struct {
	Result        string
	Detail        string
	LastChangedAt time.Time
	LastSuccessAt time.Time
	LastError     string
	LastErrorAt   time.Time
}

type Registry struct {
	mu              sync.RWMutex
	families        map[string]*family
	componentStatus map[string]ComponentStatus
	startedAt       time.Time
	rolloutSet      bool
}

func New() *Registry {
	r := &Registry{
		families:        make(map[string]*family),
		componentStatus: make(map[string]ComponentStatus),
		startedAt:       time.Now().UTC(),
	}
	r.SetGauge("tars_process_start_time_seconds", "Unix time when the current TARS process started.", nil, float64(r.startedAt.Unix()))
	return r
}

func (r *Registry) IncCounter(name string, help string, labels Labels) {
	r.AddCounter(name, help, labels, 1)
}

func (r *Registry) AddCounter(name string, help string, labels Labels, delta float64) {
	if delta == 0 {
		return
	}
	r.upsert("counter", name, help, labels, func(current float64) float64 {
		return current + delta
	})
}

func (r *Registry) SetGauge(name string, help string, labels Labels, value float64) {
	r.upsert("gauge", name, help, labels, func(_ float64) float64 {
		return value
	})
}

func (r *Registry) ObserveHistogram(name string, help string, labels Labels, buckets []float64, value float64) {
	buckets = normalizeBuckets(buckets)
	if len(buckets) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	f := r.families[name]
	if f == nil {
		f = &family{
			name:       name,
			help:       help,
			typ:        "histogram",
			histograms: make(map[string]*histogramSample),
		}
		r.families[name] = f
	}
	if f.help == "" {
		f.help = help
	}
	if f.typ == "" {
		f.typ = "histogram"
	}
	if f.histograms == nil {
		f.histograms = make(map[string]*histogramSample)
	}

	key := labelsKey(labels)
	h := f.histograms[key]
	if h == nil {
		h = &histogramSample{
			labels:  cloneLabels(labels),
			buckets: append([]float64(nil), buckets...),
			counts:  make([]uint64, len(buckets)),
		}
		f.histograms[key] = h
	}
	if len(h.buckets) != len(buckets) {
		return
	}
	for i, bucket := range h.buckets {
		if value <= bucket {
			h.counts[i]++
		}
	}
	h.sum += value
	h.count++
}

func (r *Registry) IncHTTPRequest(route string, method string, code int) {
	r.IncCounter("tars_http_requests_total", "Total HTTP requests handled by TARS.", Labels{
		"route":  route,
		"method": method,
		"code":   strconv.Itoa(code),
	})
}

func (r *Registry) AddAlertEvents(source string, result string, count int) {
	r.AddCounter("tars_alert_events_total", "Total alert events ingested by TARS.", Labels{
		"source": source,
		"result": result,
	}, float64(count))
}

func (r *Registry) IncOutbox(topic string, result string) {
	r.IncCounter("tars_outbox_events_total", "Total outbox events processed by result.", Labels{
		"topic":  topic,
		"result": result,
	})
}

func (r *Registry) IncEventBus(topic string, decision string, attempt int, reason string) {
	labels := Labels{
		"topic":    topic,
		"decision": decision,
		"attempt":  strconv.Itoa(attempt),
	}
	if strings.TrimSpace(reason) != "" {
		labels["reason"] = reason
	}
	r.IncCounter("tars_event_bus_deliveries_total", "Total event bus delivery decisions by topic and attempt.", labels)
}

func (r *Registry) IncDispatcherCycle(result string) {
	r.IncCounter("tars_dispatcher_cycles_total", "Total dispatcher worker cycles.", Labels{
		"result": result,
	})
}

func (r *Registry) IncChannelMessage(channel string, kind string, result string) {
	r.IncCounter("tars_channel_messages_total", "Total outbound channel message attempts.", Labels{
		"channel": channel,
		"kind":    kind,
		"result":  result,
	})
}

func (r *Registry) IncChannelCallback(result string) {
	r.IncCounter("tars_channel_callbacks_total", "Total inbound Telegram callback acknowledgements.", Labels{
		"result": result,
	})
}

func (r *Registry) IncExecution(status string) {
	r.IncCounter("tars_execution_requests_total", "Total execution requests by final status.", Labels{
		"status": status,
	})
}

func (r *Registry) IncExecutionOutputTruncated() {
	r.IncCounter("tars_execution_output_truncated_total", "Total execution outputs that were truncated before database persistence.", nil)
}

func (r *Registry) IncExternalProvider(provider string, operation string, result string) {
	r.IncCounter("tars_external_provider_requests_total", "Total external provider requests by provider, operation, and result.", Labels{
		"provider":  provider,
		"operation": operation,
		"result":    result,
	})
}

func (r *Registry) AddApprovalTimeouts(count int) {
	r.AddCounter("tars_approval_timeouts_total", "Total execution requests auto-rejected due to approval timeout.", nil, float64(count))
}

func (r *Registry) IncKnowledgeIngest(result string) {
	r.IncCounter("tars_knowledge_ingest_total", "Total knowledge ingest operations.", Labels{
		"result": result,
	})
}

func (r *Registry) IncKnowledgeSearch(result string) {
	r.IncCounter("tars_knowledge_search_total", "Total knowledge search requests.", Labels{
		"result": result,
	})
}

func (r *Registry) IncGCRun(result string) {
	r.IncCounter("tars_gc_runs_total", "Total garbage collector runs.", Labels{
		"result": result,
	})
}

func (r *Registry) AddGCDeleted(kind string, count int) {
	r.AddCounter("tars_gc_deleted_total", "Total objects deleted by garbage collector.", Labels{
		"kind": kind,
	}, float64(count))
}

func (r *Registry) SetRolloutMode(mode string) {
	if strings.TrimSpace(mode) == "" {
		mode = "custom"
	}
	r.SetGauge("tars_rollout_mode_info", "Current rollout mode for this TARS process.", Labels{
		"mode": mode,
	}, 1)
}

func (r *Registry) SetFeatureFlag(flag string, enabled bool) {
	value := 0.0
	if enabled {
		value = 1
	}
	r.SetGauge("tars_feature_flag_enabled", "Whether a feature flag is enabled for this TARS process.", Labels{
		"flag": flag,
	}, value)
}

func (r *Registry) RecordComponentResult(component string, result string, detail string) {
	component = strings.TrimSpace(component)
	if component == "" {
		return
	}

	now := time.Now().UTC()

	r.mu.Lock()
	defer r.mu.Unlock()

	state := r.componentStatus[component]
	state.Result = strings.TrimSpace(result)
	state.Detail = strings.TrimSpace(detail)
	state.LastChangedAt = now

	switch state.Result {
	case "success", "completed":
		state.LastSuccessAt = now
	case "error", "failed", "timeout":
		state.LastError = state.Detail
		state.LastErrorAt = now
	}

	r.componentStatus[component] = state
}

func (r *Registry) GetComponentStatus(component string) (ComponentStatus, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, ok := r.componentStatus[strings.TrimSpace(component)]
	return state, ok
}

func (r *Registry) WritePrometheus(w io.Writer) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.families))
	for name := range r.families {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		family := r.families[name]
		if _, err := fmt.Fprintf(w, "# HELP %s %s\n", family.name, family.help); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "# TYPE %s %s\n", family.name, family.typ); err != nil {
			return err
		}

		keys := make([]string, 0, len(family.samples))
		for key := range family.samples {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			s := family.samples[key]
			if len(s.labels) == 0 {
				if _, err := fmt.Fprintf(w, "%s %s\n", family.name, formatValue(s.value)); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(w, "%s{%s} %s\n", family.name, formatLabels(s.labels), formatValue(s.value)); err != nil {
				return err
			}
		}

		if family.typ != "histogram" {
			continue
		}
		hKeys := make([]string, 0, len(family.histograms))
		for key := range family.histograms {
			hKeys = append(hKeys, key)
		}
		sort.Strings(hKeys)
		for _, key := range hKeys {
			h := family.histograms[key]
			for i, bucket := range h.buckets {
				labels := cloneLabels(h.labels)
				if labels == nil {
					labels = Labels{}
				}
				labels["le"] = formatBucket(bucket)
				if _, err := fmt.Fprintf(w, "%s_bucket{%s} %s\n", family.name, formatLabels(labels), formatValue(float64(h.counts[i]))); err != nil {
					return err
				}
			}
			labels := cloneLabels(h.labels)
			if labels == nil {
				labels = Labels{}
			}
			labels["le"] = "+Inf"
			if _, err := fmt.Fprintf(w, "%s_bucket{%s} %s\n", family.name, formatLabels(labels), formatValue(float64(h.count))); err != nil {
				return err
			}
			if len(h.labels) == 0 {
				if _, err := fmt.Fprintf(w, "%s_sum %s\n", family.name, formatValue(h.sum)); err != nil {
					return err
				}
				if _, err := fmt.Fprintf(w, "%s_count %s\n", family.name, formatValue(float64(h.count))); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(w, "%s_sum{%s} %s\n", family.name, formatLabels(h.labels), formatValue(h.sum)); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "%s_count{%s} %s\n", family.name, formatLabels(h.labels), formatValue(float64(h.count))); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Registry) upsert(typ string, name string, help string, labels Labels, apply func(current float64) float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	f := r.families[name]
	if f == nil {
		f = &family{
			name:       name,
			help:       help,
			typ:        typ,
			samples:    make(map[string]*sample),
			histograms: make(map[string]*histogramSample),
		}
		r.families[name] = f
	}
	if f.help == "" {
		f.help = help
	}
	if f.typ == "" {
		f.typ = typ
	}

	key := labelsKey(labels)
	s := f.samples[key]
	if s == nil {
		s = &sample{
			labels: cloneLabels(labels),
			value:  0,
		}
		f.samples[key] = s
	}
	s.value = apply(s.value)
}

func labelsKey(labels Labels) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+labels[key])
	}
	return strings.Join(parts, "\xff")
}

func formatLabels(labels Labels) string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, key, escapeLabelValue(labels[key])))
	}
	return strings.Join(parts, ",")
}

func formatValue(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func escapeLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}

func cloneLabels(in Labels) Labels {
	if len(in) == 0 {
		return nil
	}
	out := make(Labels, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func normalizeBuckets(buckets []float64) []float64 {
	if len(buckets) == 0 {
		return nil
	}
	normalized := append([]float64(nil), buckets...)
	sort.Float64s(normalized)
	compacted := normalized[:0]
	for _, bucket := range normalized {
		if len(compacted) > 0 && compacted[len(compacted)-1] == bucket {
			continue
		}
		compacted = append(compacted, bucket)
	}
	return append([]float64(nil), compacted...)
}

func formatBucket(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
