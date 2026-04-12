package drift

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Collector aggregates drift reports over time.
// Safe for concurrent use.
type Collector struct {
	mu      sync.Mutex
	reports []Report

	// SeenFields tracks which unknown fields have been reported
	// to avoid flooding logs with the same drift.
	// Key: "provider:type:field"
	seenFields map[string]time.Time

	// OnDrift is called when new (previously unseen) drift is detected.
	// Can be nil.
	OnDrift func(Report)
}

// NewCollector creates a new drift collector.
func NewCollector(onDrift func(Report)) *Collector {
	return &Collector{
		seenFields: make(map[string]time.Time),
		OnDrift:    onDrift,
	}
}

// Record adds a drift report to the collector.
// Only triggers OnDrift for reports containing previously unseen drift fields.
func (c *Collector) Record(report *Report) {
	if report == nil || !report.HasDrift() {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	hasNew := false
	for _, f := range report.Fields {
		key := fmt.Sprintf("%s:%s:%s:%s", report.Provider, report.TypeName, f.Kind, f.Path)
		if _, seen := c.seenFields[key]; !seen {
			c.seenFields[key] = report.Timestamp
			hasNew = true
		}
	}

	c.reports = append(c.reports, *report)

	if hasNew && c.OnDrift != nil {
		c.OnDrift(*report)
	}
}

// Reports returns all collected drift reports.
func (c *Collector) Reports() []Report {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Report, len(c.reports))
	copy(out, c.reports)
	return out
}

// UnseenCount returns the number of unique drift fields seen.
func (c *Collector) UnseenCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.seenFields)
}

// SeenDriftSummary returns a summary of all unique drift fields detected.
func (c *Collector) SeenDriftSummary() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.seenFields) == 0 {
		return "no drift detected"
	}

	byProvider := map[string][]string{}
	for key := range c.seenFields {
		parts := strings.SplitN(key, ":", 4)
		if len(parts) >= 4 {
			provider := parts[0]
			byProvider[provider] = append(byProvider[provider], fmt.Sprintf("%s.%s (%s)", parts[1], parts[3], parts[2]))
		}
	}

	var lines []string
	for provider, fields := range byProvider {
		lines = append(lines, fmt.Sprintf("%s: %s", provider, strings.Join(fields, ", ")))
	}
	return strings.Join(lines, "; ")
}

// DriftSnapshot is a serializable snapshot of current drift state.
type DriftSnapshot struct {
	Timestamp    time.Time            `json:"timestamp"`
	TotalReports int                  `json:"total_reports"`
	UniqueDrifts int                  `json:"unique_drifts"`
	ByProvider   map[string][]DriftField `json:"by_provider"`
}

// Snapshot creates a serializable snapshot of current drift state.
func (c *Collector) Snapshot() DriftSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	snap := DriftSnapshot{
		Timestamp:    time.Now(),
		TotalReports: len(c.reports),
		UniqueDrifts: len(c.seenFields),
		ByProvider:   map[string][]DriftField{},
	}

	// Deduplicate fields per provider.
	seen := map[string]bool{}
	for _, report := range c.reports {
		provider := string(report.Provider)
		for _, f := range report.Fields {
			key := provider + ":" + f.Path + ":" + string(f.Kind)
			if !seen[key] {
				seen[key] = true
				snap.ByProvider[provider] = append(snap.ByProvider[provider], f)
			}
		}
	}

	return snap
}

// SnapshotJSON returns the snapshot as formatted JSON.
func (c *Collector) SnapshotJSON() ([]byte, error) {
	return json.MarshalIndent(c.Snapshot(), "", "  ")
}
