// Package metrics is a tiny, dependency-free metrics registry that renders in the
// Prometheus text exposition format. It supports labelled counters, gauges, and
// histograms — enough to expose review/agent/HTTP metrics at /api/metrics without
// pulling in the Prometheus client library.
package metrics

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const keySep = "\x1f" // unit separator: joins label values into a map key

// Registry collects metric families and renders them.
type Registry struct {
	mu         sync.Mutex
	counters   []*CounterVec
	gauges     []*GaugeVec
	histograms []*HistogramVec
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry { return &Registry{} }

// CounterVec is a monotonically increasing counter with optional labels.
type CounterVec struct {
	name, help string
	labels     []string
	mu         sync.Mutex
	series     map[string]*labeledValue
}

// GaugeVec is a value that can go up or down.
type GaugeVec struct {
	name, help string
	labels     []string
	mu         sync.Mutex
	series     map[string]*labeledValue
}

type labeledValue struct {
	labelValues []string
	value       float64
}

// HistogramVec observes a distribution into cumulative buckets.
type HistogramVec struct {
	name, help string
	labels     []string
	buckets    []float64
	mu         sync.Mutex
	series     map[string]*histValue
}

type histValue struct {
	labelValues []string
	counts      []uint64 // counts[i] = observations with value <= buckets[i] (cumulative)
	sum         float64
	count       uint64
}

// NewCounter registers a counter.
func (r *Registry) NewCounter(name, help string, labels ...string) *CounterVec {
	c := &CounterVec{name: name, help: help, labels: labels, series: map[string]*labeledValue{}}
	r.mu.Lock()
	r.counters = append(r.counters, c)
	r.mu.Unlock()
	return c
}

// NewGauge registers a gauge.
func (r *Registry) NewGauge(name, help string, labels ...string) *GaugeVec {
	g := &GaugeVec{name: name, help: help, labels: labels, series: map[string]*labeledValue{}}
	r.mu.Lock()
	r.gauges = append(r.gauges, g)
	r.mu.Unlock()
	return g
}

// NewHistogram registers a histogram with the given upper-bound buckets.
func (r *Registry) NewHistogram(name, help string, buckets []float64, labels ...string) *HistogramVec {
	h := &HistogramVec{name: name, help: help, labels: labels, buckets: buckets, series: map[string]*histValue{}}
	r.mu.Lock()
	r.histograms = append(r.histograms, h)
	r.mu.Unlock()
	return h
}

// Add increments the counter series for the given label values.
func (c *CounterVec) Add(v float64, labelValues ...string) {
	key := strings.Join(labelValues, keySep)
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.series[key]
	if s == nil {
		s = &labeledValue{labelValues: append([]string(nil), labelValues...)}
		c.series[key] = s
	}
	s.value += v
}

// Inc adds 1.
func (c *CounterVec) Inc(labelValues ...string) { c.Add(1, labelValues...) }

// Set sets the gauge series value.
func (g *GaugeVec) Set(v float64, labelValues ...string) {
	key := strings.Join(labelValues, keySep)
	g.mu.Lock()
	defer g.mu.Unlock()
	s := g.series[key]
	if s == nil {
		s = &labeledValue{labelValues: append([]string(nil), labelValues...)}
		g.series[key] = s
	}
	s.value = v
}

// Observe records a value into the histogram series.
func (h *HistogramVec) Observe(v float64, labelValues ...string) {
	key := strings.Join(labelValues, keySep)
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.series[key]
	if s == nil {
		s = &histValue{labelValues: append([]string(nil), labelValues...), counts: make([]uint64, len(h.buckets))}
		h.series[key] = s
	}
	s.sum += v
	s.count++
	for i, b := range h.buckets {
		if v <= b {
			s.counts[i]++
		}
	}
}

// Render returns the whole registry in Prometheus text exposition format.
func (r *Registry) Render() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var b strings.Builder
	for _, c := range r.counters {
		c.render(&b)
	}
	for _, g := range r.gauges {
		g.render(&b)
	}
	for _, h := range r.histograms {
		h.render(&b)
	}
	return b.String()
}

func (c *CounterVec) render(b *strings.Builder) {
	c.mu.Lock()
	defer c.mu.Unlock()
	header(b, c.name, c.help, "counter")
	for _, key := range sortedKeys(c.series) {
		s := c.series[key]
		fmt.Fprintf(b, "%s%s %s\n", c.name, labelStr(c.labels, s.labelValues), formatFloat(s.value))
	}
}

func (g *GaugeVec) render(b *strings.Builder) {
	g.mu.Lock()
	defer g.mu.Unlock()
	header(b, g.name, g.help, "gauge")
	for _, key := range sortedKeys(g.series) {
		s := g.series[key]
		fmt.Fprintf(b, "%s%s %s\n", g.name, labelStr(g.labels, s.labelValues), formatFloat(s.value))
	}
}

func (h *HistogramVec) render(b *strings.Builder) {
	h.mu.Lock()
	defer h.mu.Unlock()
	header(b, h.name, h.help, "histogram")
	keys := make([]string, 0, len(h.series))
	for k := range h.series {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		s := h.series[key]
		for i, bound := range h.buckets {
			labels := withLabel(h.labels, s.labelValues, "le", formatFloat(bound))
			fmt.Fprintf(b, "%s_bucket%s %d\n", h.name, labels, s.counts[i])
		}
		inf := withLabel(h.labels, s.labelValues, "le", "+Inf")
		fmt.Fprintf(b, "%s_bucket%s %d\n", h.name, inf, s.count)
		fmt.Fprintf(b, "%s_sum%s %s\n", h.name, labelStr(h.labels, s.labelValues), formatFloat(s.sum))
		fmt.Fprintf(b, "%s_count%s %d\n", h.name, labelStr(h.labels, s.labelValues), s.count)
	}
}

func header(b *strings.Builder, name, help, typ string) {
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, typ)
}

func sortedKeys(m map[string]*labeledValue) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// labelStr renders `{k="v",...}` for the given label names/values, or "" if none.
func labelStr(names, values []string) string {
	if len(names) == 0 {
		return ""
	}
	var parts []string
	for i, n := range names {
		v := ""
		if i < len(values) {
			v = values[i]
		}
		parts = append(parts, fmt.Sprintf("%s=%q", n, v))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// withLabel renders the base labels plus one extra (used for histogram le=).
func withLabel(names, values []string, extraKey, extraVal string) string {
	var parts []string
	for i, n := range names {
		v := ""
		if i < len(values) {
			v = values[i]
		}
		parts = append(parts, fmt.Sprintf("%s=%q", n, v))
	}
	parts = append(parts, fmt.Sprintf("%s=%q", extraKey, extraVal))
	return "{" + strings.Join(parts, ",") + "}"
}

func formatFloat(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
