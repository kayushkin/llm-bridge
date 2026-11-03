//go:build linux

package machinestats

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kayushkin/llm-bridge/msg"
)

// collectPlatform reads Linux, or Windows when this is WSL and can start
// Windows programs.
func (c *Collector) collectPlatform(ctx context.Context, stats *msg.MachineStats) {
	if runningInWSLWithInterop() {
		stats.Source = msg.MachineStatsSourceWindowsFromWSL
		collectWindowsFromWSL(ctx, stats)
		return
	}
	stats.Source = msg.MachineStatsSourceLinux
	hostname(stats)
	readLinuxOperatingSystem(stats)
	readLinuxUptime(stats)
	c.readLinuxCPU(stats)
	readLinuxMemory(stats)
	readLinuxDrives(ctx, stats)
	readLinuxBatteries(stats)
}

// runningInWSLWithInterop reports whether this is WSL and Windows programs
// can be started from it.
func runningInWSLWithInterop() bool {
	version, err := os.ReadFile("/proc/version")
	if err != nil || !strings.Contains(strings.ToLower(string(version)), "microsoft") {
		return false
	}
	_, err = os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop")
	return err == nil
}

func readLinuxOperatingSystem(stats *msg.MachineStats) {
	release, err := os.ReadFile("/etc/os-release")
	if err != nil {
		recordError(stats, "operating system", err)
	} else {
		for _, line := range strings.Split(string(release), "\n") {
			if value, found := strings.CutPrefix(line, "PRETTY_NAME="); found {
				stats.OperatingSystem = strings.Trim(value, `"`)
			}
		}
	}
	kernel, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		recordError(stats, "kernel version", err)
		return
	}
	stats.KernelVersion = strings.TrimSpace(string(kernel))
}

func readLinuxUptime(stats *msg.MachineStats) {
	uptime, err := os.ReadFile("/proc/uptime")
	if err != nil {
		recordError(stats, "uptime", err)
		return
	}
	fields := strings.Fields(string(uptime))
	if len(fields) == 0 {
		recordError(stats, "uptime", fmt.Errorf("empty /proc/uptime"))
		return
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		recordError(stats, "uptime", err)
		return
	}
	stats.UptimeSeconds = int64(seconds)
	bootedAt := stats.CollectedAt.Add(-time.Duration(seconds * float64(time.Second))).Truncate(time.Second)
	stats.BootedAt = &bootedAt
}

// readLinuxCPU reads the CPU model, core count, load average and use. Use is
// the average since the previous reading; the first reading looks over one
// second.
func (c *Collector) readLinuxCPU(stats *msg.MachineStats) {
	if information, err := os.ReadFile("/proc/cpuinfo"); err != nil {
		recordError(stats, "cpu model", err)
	} else {
		stats.CPUModel, stats.CPULogicalCores = parseLinuxCPUInformation(string(information))
	}

	if loadAverage, err := os.ReadFile("/proc/loadavg"); err != nil {
		recordError(stats, "load average", err)
	} else {
		fields := strings.Fields(string(loadAverage))
		for _, field := range fields[:min(3, len(fields))] {
			value, err := strconv.ParseFloat(field, 64)
			if err != nil {
				recordError(stats, "load average", err)
				stats.LoadAverage = nil
				break
			}
			stats.LoadAverage = append(stats.LoadAverage, value)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	current, err := readLinuxCPUTimeCounters()
	if err != nil {
		recordError(stats, "cpu usage", err)
		return
	}
	previous := c.previousCPUTime
	if previous == nil {
		time.Sleep(time.Second)
		previous = current
		if current, err = readLinuxCPUTimeCounters(); err != nil {
			recordError(stats, "cpu usage", err)
			return
		}
	}
	c.previousCPUTime = current
	if current.total <= previous.total {
		recordError(stats, "cpu usage", fmt.Errorf("cpu counters did not advance"))
		return
	}
	stats.CPUUsagePercent = float64Pointer(100 * float64(current.busy-previous.busy) / float64(current.total-previous.total))
}

// parseLinuxCPUInformation returns the model name and the number of logical
// processors listed in /proc/cpuinfo.
func parseLinuxCPUInformation(information string) (model string, logicalCores int) {
	for _, line := range strings.Split(information, "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "processor":
			logicalCores++
		case "model name", "Model":
			if model == "" {
				model = strings.TrimSpace(value)
			}
		}
	}
	return model, logicalCores
}

func readLinuxCPUTimeCounters() (*cpuTimeCounters, error) {
	file, err := os.ReadFile("/proc/stat")
	if err != nil {
		return nil, err
	}
	return parseLinuxCPUTimeCounters(string(file))
}

// parseLinuxCPUTimeCounters reads the aggregate "cpu" line of /proc/stat.
// Idle and iowait count as not busy; guest time is already inside user.
func parseLinuxCPUTimeCounters(procStat string) (*cpuTimeCounters, error) {
	for _, line := range strings.Split(procStat, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] != "cpu" {
			continue
		}
		var counters cpuTimeCounters
		// user nice system idle iowait irq softirq steal (guest, guest_nice excluded).
		for index, field := range fields[1:min(9, len(fields))] {
			value, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse /proc/stat: %w", err)
			}
			counters.total += value
			if index != 3 && index != 4 {
				counters.busy += value
			}
		}
		return &counters, nil
	}
	return nil, fmt.Errorf("no cpu line in /proc/stat")
}

func readLinuxMemory(stats *msg.MachineStats) {
	file, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		recordError(stats, "memory", err)
		return
	}
	values, err := parseLinuxMemoryInformation(string(file))
	if err != nil {
		recordError(stats, "memory", err)
		return
	}
	for _, key := range []string{"MemTotal", "MemAvailable", "SwapTotal", "SwapFree"} {
		if _, found := values[key]; !found {
			recordError(stats, "memory", fmt.Errorf("/proc/meminfo has no %s", key))
			return
		}
	}
	stats.MemoryTotalBytes = values["MemTotal"]
	stats.MemoryAvailableBytes = values["MemAvailable"]
	stats.MemoryUsedBytes = values["MemTotal"] - values["MemAvailable"]
	stats.SwapTotalBytes = values["SwapTotal"]
	stats.SwapUsedBytes = values["SwapTotal"] - values["SwapFree"]
}

// parseLinuxMemoryInformation returns /proc/meminfo's values in bytes.
func parseLinuxMemoryInformation(memoryInformation string) (map[string]uint64, error) {
	values := map[string]uint64{}
	for _, line := range strings.Split(memoryInformation, "\n") {
		key, rest, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		value, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", key, err)
		}
		if len(fields) > 1 && fields[1] == "kB" {
			value *= 1024
		}
		values[key] = value
	}
	return values, nil
}

// networkAndPooledFilesystemTypes are file systems with no /dev block device
// that still hold real data: pools and network shares.
var networkAndPooledFilesystemTypes = map[string]bool{
	"zfs": true, "nfs": true, "nfs4": true, "cifs": true, "smb3": true,
	"fuse.mergerfs": true, "fuse.sshfs": true,
}

// linuxMount is one line of /proc/mounts.
type linuxMount struct {
	device, mountPoint, filesystemType string
}

// selectLinuxDriveMounts keeps mounts that hold data — block devices other
// than loop devices (snaps), and pools and network shares — once per device,
// at its shortest mount point, so bind mounts do not repeat a drive.
func selectLinuxDriveMounts(procMounts string) []linuxMount {
	byDevice := map[string]linuxMount{}
	for _, line := range strings.Split(procMounts, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		mount := linuxMount{device: fields[0], mountPoint: unescapeMountField(fields[1]), filesystemType: fields[2]}
		blockDevice := strings.HasPrefix(mount.device, "/dev/") && !strings.HasPrefix(mount.device, "/dev/loop")
		if !blockDevice && !networkAndPooledFilesystemTypes[mount.filesystemType] {
			continue
		}
		if mount.filesystemType == "squashfs" {
			continue
		}
		if existing, found := byDevice[mount.device]; found && len(existing.mountPoint) <= len(mount.mountPoint) {
			continue
		}
		byDevice[mount.device] = mount
	}
	mounts := make([]linuxMount, 0, len(byDevice))
	for _, mount := range byDevice {
		mounts = append(mounts, mount)
	}
	sort.Slice(mounts, func(i, j int) bool { return mounts[i].mountPoint < mounts[j].mountPoint })
	return mounts
}

// unescapeMountField undoes /proc/mounts' octal escapes (\040 for a space).
func unescapeMountField(field string) string {
	if !strings.Contains(field, `\`) {
		return field
	}
	var builder strings.Builder
	for index := 0; index < len(field); index++ {
		if field[index] == '\\' && index+3 < len(field) {
			if value, err := strconv.ParseUint(field[index+1:index+4], 8, 8); err == nil {
				builder.WriteByte(byte(value))
				index += 3
				continue
			}
		}
		builder.WriteByte(field[index])
	}
	return builder.String()
}

func readLinuxDrives(ctx context.Context, stats *msg.MachineStats) {
	procMounts, err := os.ReadFile("/proc/mounts")
	if err != nil {
		recordError(stats, "drives", err)
		return
	}
	for _, mount := range selectLinuxDriveMounts(string(procMounts)) {
		drive, err := statLinuxMount(ctx, mount)
		if err != nil {
			recordError(stats, "drive "+mount.mountPoint, err)
			continue
		}
		stats.Drives = append(stats.Drives, drive)
	}
}

// statLinuxMount asks the kernel for a mount's size. A dead network share can
// block statfs for minutes, so it gives up after commandTimeout.
func statLinuxMount(ctx context.Context, mount linuxMount) (msg.MachineDrive, error) {
	type result struct {
		filesystem syscall.Statfs_t
		err        error
	}
	done := make(chan result, 1)
	go func() {
		var filesystem syscall.Statfs_t
		err := syscall.Statfs(mount.mountPoint, &filesystem)
		done <- result{filesystem, err}
	}()
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	select {
	case <-ctx.Done():
		return msg.MachineDrive{}, fmt.Errorf("statfs: %w", ctx.Err())
	case outcome := <-done:
		if outcome.err != nil {
			return msg.MachineDrive{}, fmt.Errorf("statfs: %w", outcome.err)
		}
		blockSize := uint64(outcome.filesystem.Bsize)
		return msg.MachineDrive{
			MountPoint:     mount.mountPoint,
			Device:         mount.device,
			FilesystemType: mount.filesystemType,
			TotalBytes:     outcome.filesystem.Blocks * blockSize,
			UsedBytes:      (outcome.filesystem.Blocks - outcome.filesystem.Bfree) * blockSize,
			FreeBytes:      outcome.filesystem.Bavail * blockSize,
		}, nil
	}
}

// linuxPowerSupplyDirectory is where the kernel lists batteries and chargers.
var linuxPowerSupplyDirectory = "/sys/class/power_supply"

func readLinuxBatteries(stats *msg.MachineStats) {
	entries, err := os.ReadDir(linuxPowerSupplyDirectory)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		recordError(stats, "batteries", err)
		return
	}
	for _, entry := range entries {
		directory := filepath.Join(linuxPowerSupplyDirectory, entry.Name())
		if readPowerSupplyText(directory, "type") != "Battery" {
			continue
		}
		battery, err := readLinuxBattery(entry.Name(), directory)
		if err != nil {
			recordError(stats, "battery "+entry.Name(), err)
			continue
		}
		stats.Batteries = append(stats.Batteries, battery)
	}
}

func readPowerSupplyText(directory, name string) string {
	value, err := os.ReadFile(filepath.Join(directory, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(value))
}

// readPowerSupplyNumber returns a numeric attribute, or false when the driver
// does not provide it.
func readPowerSupplyNumber(directory, name string) (int64, bool) {
	value, err := strconv.ParseInt(readPowerSupplyText(directory, name), 10, 64)
	return value, err == nil
}

// readLinuxBattery reads one battery. Drivers report either energy (µWh,
// power in µW) or charge (µAh, current in µA); the arithmetic is the same.
func readLinuxBattery(name, directory string) (msg.MachineBattery, error) {
	battery := msg.MachineBattery{Name: name}
	capacity, found := readPowerSupplyNumber(directory, "capacity")
	if !found {
		return battery, fmt.Errorf("no capacity")
	}
	battery.ChargePercent = float64(capacity)
	switch readPowerSupplyText(directory, "status") {
	case "Charging":
		battery.State = msg.MachineBatteryCharging
	case "Discharging":
		battery.State = msg.MachineBatteryDischarging
	case "Full":
		battery.State = msg.MachineBatteryFull
	case "Not charging":
		battery.State = msg.MachineBatteryNotCharging
	default:
		battery.State = msg.MachineBatteryUnknown
	}

	for _, prefix := range []string{"energy", "charge"} {
		now, hasNow := readPowerSupplyNumber(directory, prefix+"_now")
		full, hasFull := readPowerSupplyNumber(directory, prefix+"_full")
		design, hasDesign := readPowerSupplyNumber(directory, prefix+"_full_design")
		if !hasNow && !hasFull {
			continue
		}
		if hasFull && hasDesign && design > 0 {
			battery.HealthPercent = float64Pointer(100 * float64(full) / float64(design))
		}
		rateName := "power_now"
		if prefix == "charge" {
			rateName = "current_now"
		}
		rate, hasRate := readPowerSupplyNumber(directory, rateName)
		if hasRate && rate < 0 {
			rate = -rate
		}
		if hasRate && rate > 0 && hasNow {
			switch battery.State {
			case msg.MachineBatteryDischarging:
				battery.SecondsRemaining = int64Pointer(now * 3600 / rate)
			case msg.MachineBatteryCharging:
				if hasFull && full > now {
					battery.SecondsRemaining = int64Pointer((full - now) * 3600 / rate)
				}
			}
		}
		break
	}
	if cycles, found := readPowerSupplyNumber(directory, "cycle_count"); found && cycles > 0 {
		battery.CycleCount = int64Pointer(cycles)
	}
	return battery, nil
}
