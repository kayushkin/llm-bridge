package machinestats

// The macOS tools' output is parsed here, outside darwin.go's build tag, so
// the parsing is tested on every OS.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kayushkin/llm-bridge/msg"
)

var darwinBootTimePattern = regexp.MustCompile(`sec = (\d+)`)

func parseDarwinBootTime(value string) (time.Time, error) {
	match := darwinBootTimePattern.FindStringSubmatch(value)
	if match == nil {
		return time.Time{}, fmt.Errorf("unexpected kern.boottime %q", value)
	}
	seconds, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(seconds, 0).UTC(), nil
}

var darwinSwapPattern = regexp.MustCompile(`total = ([\d.]+)([KMG])\s+used = ([\d.]+)([KMG])`)

func parseDarwinSwapUsage(value string) (total, used uint64, err error) {
	match := darwinSwapPattern.FindStringSubmatch(value)
	if match == nil {
		return 0, 0, fmt.Errorf("unexpected vm.swapusage %q", value)
	}
	toBytes := func(number, unit string) (uint64, error) {
		parsed, err := strconv.ParseFloat(number, 64)
		if err != nil {
			return 0, err
		}
		multiplier := map[string]float64{"K": 1 << 10, "M": 1 << 20, "G": 1 << 30}[unit]
		return uint64(parsed * multiplier), nil
	}
	if total, err = toBytes(match[1], match[2]); err != nil {
		return 0, 0, err
	}
	if used, err = toBytes(match[3], match[4]); err != nil {
		return 0, 0, err
	}
	return total, used, nil
}

var darwinPageSizePattern = regexp.MustCompile(`page size of (\d+) bytes`)

func parseDarwinMemoryUsed(vmStat string) (uint64, error) {
	match := darwinPageSizePattern.FindStringSubmatch(vmStat)
	if match == nil {
		return 0, fmt.Errorf("vm_stat gave no page size")
	}
	pageSize, err := strconv.ParseUint(match[1], 10, 64)
	if err != nil {
		return 0, err
	}
	pages := map[string]uint64{}
	for _, line := range strings.Split(vmStat, "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		count, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(value), "."), 10, 64)
		if err != nil {
			continue
		}
		pages[strings.TrimSpace(key)] = count
	}
	needed := []string{"Pages active", "Pages wired down", "Pages occupied by compressor"}
	var usedPages uint64
	for _, key := range needed {
		count, found := pages[key]
		if !found {
			return 0, fmt.Errorf("vm_stat gave no %q", key)
		}
		usedPages += count
	}
	return usedPages * pageSize, nil
}

var darwinCPUIdlePattern = regexp.MustCompile(`CPU usage: .*?([\d.]+)% idle`)

func parseDarwinDiskFree(diskFree string) ([]msg.MachineDrive, error) {
	var drives []msg.MachineDrive
	for _, line := range strings.Split(diskFree, "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < 6 || !strings.HasPrefix(fields[0], "/dev/") {
			continue
		}
		// The mount point is the rest of the line, and may hold spaces.
		mountPoint := strings.Join(fields[5:], " ")
		if mountPoint != "/" && !strings.HasPrefix(mountPoint, "/Volumes/") {
			continue
		}
		totalKilobytes, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse df size %q: %w", fields[1], err)
		}
		freeKilobytes, err := strconv.ParseUint(fields[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse df available %q: %w", fields[3], err)
		}
		drive := msg.MachineDrive{
			MountPoint: mountPoint,
			Device:     fields[0],
			TotalBytes: totalKilobytes * 1024,
			FreeBytes:  freeKilobytes * 1024,
		}
		drive.UsedBytes = drive.TotalBytes - min(drive.FreeBytes, drive.TotalBytes)
		if label, found := strings.CutPrefix(mountPoint, "/Volumes/"); found {
			drive.Label = label
		}
		drives = append(drives, drive)
	}
	return drives, nil
}

var darwinBatteryLinePattern = regexp.MustCompile(`-(\S+) .*?\t(\d+)%; ([^;]+);\s*(\S+ remaining|\(no estimate\))?`)

func parseDarwinPowerManagementBatteries(output string) ([]msg.MachineBattery, error) {
	var batteries []msg.MachineBattery
	for _, line := range strings.Split(output, "\n") {
		match := darwinBatteryLinePattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		charge, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			return nil, err
		}
		battery := msg.MachineBattery{Name: match[1], ChargePercent: charge}
		switch strings.TrimSpace(match[3]) {
		case "charging", "finishing charge":
			battery.State = msg.MachineBatteryCharging
		case "discharging":
			battery.State = msg.MachineBatteryDischarging
		case "charged":
			battery.State = msg.MachineBatteryFull
		case "AC attached":
			battery.State = msg.MachineBatteryNotCharging
		default:
			battery.State = msg.MachineBatteryUnknown
		}
		if remaining, found := strings.CutSuffix(match[4], " remaining"); found {
			hours, minutes, found := strings.Cut(remaining, ":")
			hoursValue, hoursErr := strconv.ParseInt(hours, 10, 64)
			minutesValue, minutesErr := strconv.ParseInt(minutes, 10, 64)
			if found && hoursErr == nil && minutesErr == nil && hoursValue+minutesValue > 0 {
				battery.SecondsRemaining = int64Pointer(hoursValue*3600 + minutesValue*60)
			}
		}
		batteries = append(batteries, battery)
	}
	return batteries, nil
}

var darwinRegistryNumberPattern = regexp.MustCompile(`"(AppleRawMaxCapacity|DesignCapacity|CycleCount)" = (\d+)`)

func parseDarwinBatteryRegistry(registry string) (health *float64, cycles *int64) {
	values := map[string]int64{}
	for _, match := range darwinRegistryNumberPattern.FindAllStringSubmatch(registry, -1) {
		if _, seen := values[match[1]]; seen {
			continue
		}
		value, err := strconv.ParseInt(match[2], 10, 64)
		if err == nil {
			values[match[1]] = value
		}
	}
	if raw, design := values["AppleRawMaxCapacity"], values["DesignCapacity"]; raw > 0 && design > 0 {
		health = float64Pointer(100 * float64(raw) / float64(design))
	}
	if count, found := values["CycleCount"]; found {
		cycles = int64Pointer(count)
	}
	return health, cycles
}
