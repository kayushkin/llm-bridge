//go:build !linux && !darwin

package machinestats

import (
	"context"
	"fmt"
	"runtime"

	"github.com/kayushkin/llm-bridge/msg"
)

// collectPlatform has no reader for this OS; it says so in the sample.
func (c *Collector) collectPlatform(ctx context.Context, stats *msg.MachineStats) {
	hostname(stats)
	recordError(stats, "machine stats", fmt.Errorf("no reader for %s", runtime.GOOS))
}
