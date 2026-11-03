//go:build darwin

package machinestats

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/kayushkin/llm-bridge/msg"
)

// collectPlatform reads macOS through the tools it ships with; the runner is
// built without cgo, so the Mach calls those tools make are out of reach.
func (c *Collector) collectPlatform(ctx context.Context, stats *msg.MachineStats) {
	stats.Source = msg.MachineStatsSourceDarwin
	hostname(stats)
	readDarwinOperatingSystem(ctx, stats)
	readDarwinSysctl(ctx, stats)
	readDarwinMemory(ctx, stats)
	readDarwinCPUUsage(ctx, stats)
	readDarwinDrives(ctx, stats)
	readDarwinBatteries(ctx, stats)
}

func readDarwinOperatingSystem(ctx context.Context, stats *msg.MachineStats) {
	name, nameErr := runCommand(ctx, "", "sw_vers", "-productName")
	version, versionErr := runCommand(ctx, "", "sw_vers", "-productVersion")
	if nameErr != nil || versionErr != nil {
		recordError(stats, "operating system", fmt.Errorf("%v %v", nameErr, versionErr))
	} else {
		stats.OperatingSystem = strings.TrimSpace(name) + " " + strings.TrimSpace(version)
	}
	kernel, err := runCommand(ctx, "", "uname", "-r")
	if err != nil {
		recordError(stats, "kernel version", err)
		return
	}
	stats.KernelVersion = strings.TrimSpace(kernel)
}

// readDarwinSysctl reads boot time, CPU, load and memory size and swap.
func readDarwinSysctl(ctx context.Context, stats *msg.MachineStats) {
	keys := []string{"kern.boottime", "machdep.cpu.brand_string", "hw.logicalcpu", "hw.memsize", "vm.loadavg", "vm.swapusage"}
	values := map[string]string{}
	for _, key := range keys {
		output, err := runCommand(ctx, "", "sysctl", "-n", key)
		if err != nil {
			recordError(stats, key, err)
			continue
		}
		values[key] = strings.TrimSpace(output)
	}
	if value, found := values["kern.boottime"]; found {
		bootedAt, err := parseDarwinBootTime(value)
		if err != nil {
			recordError(stats, "uptime", err)
		} else {
			stats.BootedAt = &bootedAt
			stats.UptimeSeconds = int64(stats.CollectedAt.Sub(bootedAt).Seconds())
		}
	}
	stats.CPUModel = values["machdep.cpu.brand_string"]
	if value, found := values["hw.logicalcpu"]; found {
		cores, err := strconv.Atoi(value)
		if err != nil {
			recordError(stats, "cpu cores", err)
		}
		stats.CPULogicalCores = cores
	}
	if value, found := values["hw.memsize"]; found {
		total, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			recordError(stats, "memory", err)
		}
		stats.MemoryTotalBytes = total
	}
	if value, found := values["vm.loadavg"]; found {
		for _, field := range strings.Fields(strings.Trim(value, "{} ")) {
			load, err := strconv.ParseFloat(field, 64)
			if err != nil {
				recordError(stats, "load average", err)
				stats.LoadAverage = nil
				break
			}
			stats.LoadAverage = append(stats.LoadAverage, load)
		}
	}
	if value, found := values["vm.swapusage"]; found {
		total, used, err := parseDarwinSwapUsage(value)
		if err != nil {
			recordError(stats, "swap", err)
		} else {
			stats.SwapTotalBytes, stats.SwapUsedBytes = total, used
		}
	}
}

// parseDarwinBootTime reads "{ sec = 1730000000, usec = 123 } Mon Oct 27 …".
// parseDarwinSwapUsage reads "total = 2048.00M  used = 1031.25M  free = …".
func readDarwinMemory(ctx context.Context, stats *msg.MachineStats) {
	output, err := runCommand(ctx, "", "vm_stat")
	if err != nil {
		recordError(stats, "memory", err)
		return
	}
	used, err := parseDarwinMemoryUsed(output)
	if err != nil {
		recordError(stats, "memory", err)
		return
	}
	stats.MemoryUsedBytes = used
	if stats.MemoryTotalBytes > used {
		stats.MemoryAvailableBytes = stats.MemoryTotalBytes - used
	}
}

// parseDarwinMemoryUsed reads vm_stat and counts memory as Activity Monitor
// does for "Memory Used": app memory (active pages), wired pages, and pages
// held by the compressor.
// readDarwinCPUUsage asks top for two samples a second apart and keeps the
// second; the first covers the time since boot.
func readDarwinCPUUsage(ctx context.Context, stats *msg.MachineStats) {
	output, err := runCommand(ctx, "", "top", "-l", "2", "-n", "0", "-s", "1")
	if err != nil {
		recordError(stats, "cpu usage", err)
		return
	}
	matches := darwinCPUIdlePattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		recordError(stats, "cpu usage", fmt.Errorf("top gave no CPU usage line"))
		return
	}
	idle, err := strconv.ParseFloat(matches[len(matches)-1][1], 64)
	if err != nil {
		recordError(stats, "cpu usage", err)
		return
	}
	stats.CPUUsagePercent = float64Pointer(100 - idle)
}

// readDarwinDrives reads local file systems with df. APFS volumes in one
// container share its space, so each volume's own "used" says little; used is
// the container's size less its free space. The system's helper volumes under
// /System/Volumes repeat the startup disk and are left out.
func readDarwinDrives(ctx context.Context, stats *msg.MachineStats) {
	output, err := runCommand(ctx, "", "df", "-kPl")
	if err != nil {
		recordError(stats, "drives", err)
		return
	}
	drives, err := parseDarwinDiskFree(output)
	if err != nil {
		recordError(stats, "drives", err)
		return
	}
	stats.Drives = drives
}

func readDarwinBatteries(ctx context.Context, stats *msg.MachineStats) {
	output, err := runCommand(ctx, "", "pmset", "-g", "batt")
	if err != nil {
		recordError(stats, "batteries", err)
		return
	}
	batteries, err := parseDarwinPowerManagementBatteries(output)
	if err != nil {
		recordError(stats, "batteries", err)
		return
	}
	if len(batteries) == 0 {
		return
	}
	registry, err := runCommand(ctx, "", "ioreg", "-rn", "AppleSmartBattery")
	if err != nil {
		recordError(stats, "battery health", err)
	} else {
		health, cycles := parseDarwinBatteryRegistry(registry)
		batteries[0].HealthPercent = health
		batteries[0].CycleCount = cycles
	}
	stats.Batteries = batteries
}

// parseDarwinPowerManagementBatteries reads `pmset -g batt`:
//
//	-InternalBattery-0 (id=4653155)	85%; discharging; 4:12 remaining present: true
// parseDarwinBatteryRegistry reads health and cycles from the battery's I/O
// Kit entry. Health is the raw full-charge capacity over design capacity, in
// mAh; on Apple silicon "MaxCapacity" is a percentage of the current maximum
// and says nothing about wear.
