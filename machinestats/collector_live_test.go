package machinestats

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

// TestCollectThisMachine reads the machine the test runs on. Set
// MACHINESTATS_PRINT=1 to see the reading.
func TestCollectThisMachine(t *testing.T) {
	stats := NewCollector().Collect(context.Background())
	if os.Getenv("MACHINESTATS_PRINT") != "" {
		encoded, _ := json.MarshalIndent(stats, "", "  ")
		t.Log(string(encoded))
	}
	if stats.CollectedAt.IsZero() || stats.Source == "" {
		t.Fatalf("reading has no time or source: %+v", stats)
	}
}
