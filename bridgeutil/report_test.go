package bridgeutil

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestCollector_RecordNoDrift(t *testing.T) {
	called := false
	c := NewCollector(func(Report) { called = true })

	// nil report
	c.Record(nil)

	// Empty report
	c.Record(&Report{Provider: ProviderAnthropic})

	if called {
		t.Error("OnDrift should not be called for non-drift reports")
	}
	if len(c.Reports()) != 0 {
		t.Error("should have 0 reports")
	}
}

func TestCollector_RecordNewDrift(t *testing.T) {
	var received []Report
	c := NewCollector(func(r Report) { received = append(received, r) })

	report := &Report{
		Provider:  ProviderOpenAI,
		TypeName:  "chat_completion",
		Timestamp: time.Now(),
		Fields: []DriftField{
			{Kind: DriftUnknownField, Path: "new_field", Field: "new_field"},
		},
	}
	c.Record(report)

	if len(received) != 1 {
		t.Fatalf("expected 1 callback, got %d", len(received))
	}
	if len(c.Reports()) != 1 {
		t.Fatalf("expected 1 report, got %d", len(c.Reports()))
	}
}

func TestCollector_DeduplicatesSameDrift(t *testing.T) {
	callCount := 0
	c := NewCollector(func(Report) { callCount++ })

	report := &Report{
		Provider:  ProviderAnthropic,
		TypeName:  "message_response",
		Timestamp: time.Now(),
		Fields: []DriftField{
			{Kind: DriftUnknownField, Path: "surprise", Field: "surprise"},
		},
	}

	c.Record(report)
	c.Record(report) // Same drift again

	if callCount != 1 {
		t.Errorf("OnDrift called %d times, expected 1 (should deduplicate)", callCount)
	}
	// Both reports stored, but only one callback
	if len(c.Reports()) != 2 {
		t.Errorf("expected 2 reports stored, got %d", len(c.Reports()))
	}
}

func TestCollector_DifferentDriftsCallBack(t *testing.T) {
	callCount := 0
	c := NewCollector(func(Report) { callCount++ })

	c.Record(&Report{
		Provider: ProviderOpenAI, TypeName: "chat_completion", Timestamp: time.Now(),
		Fields: []DriftField{{Kind: DriftUnknownField, Path: "field_a", Field: "field_a"}},
	})
	c.Record(&Report{
		Provider: ProviderOpenAI, TypeName: "chat_completion", Timestamp: time.Now(),
		Fields: []DriftField{{Kind: DriftUnknownField, Path: "field_b", Field: "field_b"}},
	})

	if callCount != 2 {
		t.Errorf("OnDrift called %d times, expected 2 for different fields", callCount)
	}
}

func TestCollector_UnseenCount(t *testing.T) {
	c := NewCollector(nil)

	c.Record(&Report{
		Provider: ProviderAnthropic, TypeName: "test", Timestamp: time.Now(),
		Fields: []DriftField{
			{Kind: DriftUnknownField, Path: "a", Field: "a"},
			{Kind: DriftMissingField, Path: "b", Field: "b"},
		},
	})

	if c.UnseenCount() != 2 {
		t.Errorf("UnseenCount = %d, want 2", c.UnseenCount())
	}
}

func TestCollector_SeenDriftSummary_NoDrift(t *testing.T) {
	c := NewCollector(nil)
	if s := c.SeenDriftSummary(); s != "no drift detected" {
		t.Errorf("Summary = %q", s)
	}
}

func TestCollector_SeenDriftSummary_WithDrift(t *testing.T) {
	c := NewCollector(nil)
	c.Record(&Report{
		Provider: ProviderAnthropic, TypeName: "message_response", Timestamp: time.Now(),
		Fields: []DriftField{{Kind: DriftUnknownField, Path: "new_field", Field: "new_field"}},
	})
	s := c.SeenDriftSummary()
	if s == "no drift detected" {
		t.Error("expected non-empty summary")
	}
}

func TestCollector_NilCallback(t *testing.T) {
	c := NewCollector(nil)
	// Should not panic with nil callback
	c.Record(&Report{
		Provider: ProviderAnthropic, TypeName: "test", Timestamp: time.Now(),
		Fields: []DriftField{{Kind: DriftUnknownField, Path: "x", Field: "x"}},
	})
	if c.UnseenCount() != 1 {
		t.Error("should still track drift with nil callback")
	}
}

// --- Snapshot ---

func TestCollector_Snapshot(t *testing.T) {
	c := NewCollector(nil)

	c.Record(&Report{
		Provider: ProviderAnthropic, TypeName: "message_response", Timestamp: time.Now(),
		Fields: []DriftField{{Kind: DriftUnknownField, Path: "field_a", Field: "field_a"}},
	})
	c.Record(&Report{
		Provider: ProviderOpenAI, TypeName: "chat_completion", Timestamp: time.Now(),
		Fields: []DriftField{{Kind: DriftMissingField, Path: "field_b", Field: "field_b"}},
	})

	snap := c.Snapshot()
	if snap.TotalReports != 2 {
		t.Errorf("TotalReports = %d, want 2", snap.TotalReports)
	}
	if snap.UniqueDrifts != 2 {
		t.Errorf("UniqueDrifts = %d, want 2", snap.UniqueDrifts)
	}
	if len(snap.ByProvider) != 2 {
		t.Errorf("ByProvider count = %d, want 2", len(snap.ByProvider))
	}
}

func TestCollector_SnapshotJSON(t *testing.T) {
	c := NewCollector(nil)
	c.Record(&Report{
		Provider: ProviderAnthropic, TypeName: "test", Timestamp: time.Now(),
		Fields: []DriftField{{Kind: DriftUnknownField, Path: "x", Field: "x"}},
	})

	data, err := c.SnapshotJSON()
	if err != nil {
		t.Fatalf("SnapshotJSON: %v", err)
	}
	if !json.Valid(data) {
		t.Error("SnapshotJSON produced invalid JSON")
	}
}

// --- Concurrency ---

func TestCollector_ConcurrentRecord(t *testing.T) {
	c := NewCollector(nil)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.Record(&Report{
				Provider: ProviderAnthropic, TypeName: "test", Timestamp: time.Now(),
				Fields: []DriftField{{Kind: DriftUnknownField, Path: "x", Field: "x"}},
			})
		}(i)
	}
	wg.Wait()

	if len(c.Reports()) != 50 {
		t.Errorf("expected 50 reports, got %d", len(c.Reports()))
	}
	// All same field, so unseen count should be 1
	if c.UnseenCount() != 1 {
		t.Errorf("UnseenCount = %d, want 1", c.UnseenCount())
	}
}
