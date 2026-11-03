// Package machinestats reads a machine's health — CPU, memory, drives,
// batteries, uptime — into a msg.MachineStats. llm-bridge-runner sends a
// reading every minute; llm-bridge-server reads its own machine the same way.
//
// It uses only the standard library and the tools each OS ships with, so
// llm-bridge keeps no dependencies: /proc and /sys on Linux; sysctl, vm_stat,
// df, top, pmset and ioreg on macOS; and in WSL one PowerShell call, because
// a runner there should report the Windows machine, not the Linux virtual
// machine WSL runs it in.
package machinestats

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/kayushkin/llm-bridge/msg"
)

// SampleInterval is how often a runner and the bridge take a reading.
const SampleInterval = time.Minute

// commandTimeout bounds each tool a reading runs, so a hung mount or a slow
// PowerShell start costs one part of one sample, not the sampler.
const commandTimeout = 30 * time.Second

// Collector takes readings. It keeps the previous CPU counters so that, where
// the OS exposes counters, CPU use is the average since the previous reading.
// Safe for concurrent use.
type Collector struct {
	mu              sync.Mutex
	previousCPUTime *cpuTimeCounters
}

// NewCollector returns a Collector with no previous reading.
func NewCollector() *Collector { return &Collector{} }

// Collect takes one reading. It never fails as a whole: a part it cannot read
// is left empty and named in CollectionErrors.
func (c *Collector) Collect(ctx context.Context) msg.MachineStats {
	stats := msg.MachineStats{CollectedAt: time.Now().UTC()}
	c.collectPlatform(ctx, &stats)
	return stats
}

// cpuTimeCounters are cumulative busy and total CPU ticks.
type cpuTimeCounters struct {
	busy  uint64
	total uint64
}

// recordError notes a part that could not be read.
func recordError(stats *msg.MachineStats, part string, err error) {
	stats.CollectionErrors = append(stats.CollectionErrors, fmt.Sprintf("%s: %v", part, err))
}

// runCommand runs a tool and returns its standard output, with its standard
// error in the error when it fails.
func runCommand(ctx context.Context, directory string, name string, arguments ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, name, arguments...)
	command.Dir = directory
	var standardError strings.Builder
	command.Stderr = &standardError
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(standardError.String()))
	}
	return string(output), nil
}

// hostname fills Hostname from the OS.
func hostname(stats *msg.MachineStats) {
	name, err := os.Hostname()
	if err != nil {
		recordError(stats, "hostname", err)
		return
	}
	stats.Hostname = name
}

func float64Pointer(value float64) *float64 { return &value }
func int64Pointer(value int64) *int64       { return &value }
