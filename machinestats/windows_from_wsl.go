//go:build linux

package machinestats

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kayushkin/llm-bridge/msg"
)

// windowsStatsScript asks Windows for everything in one PowerShell start,
// because each start costs a second or more. Battery health and cycle count
// come from root\wmi classes that some drivers do not fill; their absence is
// not an error, so those reads are allowed to come back empty.
const windowsStatsScript = `
$ErrorActionPreference = 'Stop'
$os = Get-CimInstance Win32_OperatingSystem
$processors = @(Get-CimInstance Win32_Processor)
$disks = @(Get-CimInstance Win32_LogicalDisk -Filter 'DriveType=3' | ForEach-Object {
  [pscustomobject]@{ device_id = $_.DeviceID; volume_name = $_.VolumeName; file_system = $_.FileSystem; size = [uint64]$_.Size; free_space = [uint64]$_.FreeSpace } })
$batteries = @(Get-CimInstance Win32_Battery | ForEach-Object {
  [pscustomobject]@{ name = $_.Name; device_id = $_.DeviceID; charge_percent = $_.EstimatedChargeRemaining; status = $_.BatteryStatus; run_time_minutes = $_.EstimatedRunTime } })
$fullCapacities = @(try { Get-CimInstance -Namespace root\wmi BatteryFullChargedCapacity | ForEach-Object { $_.FullChargedCapacity } } catch { })
$designCapacities = @(try { Get-CimInstance -Namespace root\wmi BatteryStaticData | ForEach-Object { $_.DesignedCapacity } } catch { })
$cycleCounts = @(try { Get-CimInstance -Namespace root\wmi BatteryCycleCount | ForEach-Object { $_.CycleCount } } catch { })
$pageFiles = @(Get-CimInstance Win32_PageFileUsage | ForEach-Object {
  [pscustomobject]@{ allocated_megabytes = $_.AllocatedBaseSize; used_megabytes = $_.CurrentUsage } })
[pscustomobject]@{
  hostname = $os.CSName
  caption = $os.Caption
  version = $os.Version
  booted_at = $os.LastBootUpTime.ToUniversalTime().ToString('o')
  total_visible_memory_kilobytes = [uint64]$os.TotalVisibleMemorySize
  free_physical_memory_kilobytes = [uint64]$os.FreePhysicalMemory
  processors = @($processors | ForEach-Object { [pscustomobject]@{ name = $_.Name.Trim(); logical_processors = $_.NumberOfLogicalProcessors; load_percentage = $_.LoadPercentage } })
  disks = $disks
  batteries = $batteries
  full_charged_capacities = $fullCapacities
  designed_capacities = $designCapacities
  cycle_counts = $cycleCounts
  page_files = $pageFiles
} | ConvertTo-Json -Depth 4 -Compress
`

// windowsStats is what windowsStatsScript prints.
type windowsStats struct {
	Hostname                    string `json:"hostname"`
	Caption                     string `json:"caption"`
	Version                     string `json:"version"`
	BootedAt                    string `json:"booted_at"`
	TotalVisibleMemoryKilobytes uint64 `json:"total_visible_memory_kilobytes"`
	FreePhysicalMemoryKilobytes uint64 `json:"free_physical_memory_kilobytes"`
	Processors                  []struct {
		Name              string   `json:"name"`
		LogicalProcessors int      `json:"logical_processors"`
		LoadPercentage    *float64 `json:"load_percentage"`
	} `json:"processors"`
	Disks []struct {
		DeviceID   string `json:"device_id"`
		VolumeName string `json:"volume_name"`
		FileSystem string `json:"file_system"`
		Size       uint64 `json:"size"`
		FreeSpace  uint64 `json:"free_space"`
	} `json:"disks"`
	Batteries []struct {
		Name           string `json:"name"`
		DeviceID       string `json:"device_id"`
		ChargePercent  *int64 `json:"charge_percent"`
		Status         *int64 `json:"status"`
		RunTimeMinutes *int64 `json:"run_time_minutes"`
	} `json:"batteries"`
	FullChargedCapacities []int64 `json:"full_charged_capacities"`
	DesignedCapacities    []int64 `json:"designed_capacities"`
	CycleCounts           []int64 `json:"cycle_counts"`
	PageFiles             []struct {
		AllocatedMegabytes uint64 `json:"allocated_megabytes"`
		UsedMegabytes      uint64 `json:"used_megabytes"`
	} `json:"page_files"`
}

// windowsRunTimeUnknown is what Win32_Battery.EstimatedRunTime holds when
// Windows has no estimate (on mains power, mostly).
const windowsRunTimeUnknown = 71582788

func collectWindowsFromWSL(ctx context.Context, stats *msg.MachineStats) {
	// powershell.exe complains when started in a Linux directory.
	output, err := runCommand(ctx, "/mnt/c", "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", windowsStatsScript)
	if err != nil {
		recordError(stats, "windows", err)
		return
	}
	var windows windowsStats
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &windows); err != nil {
		recordError(stats, "windows", fmt.Errorf("parse PowerShell output: %w", err))
		return
	}
	applyWindowsStats(&windows, stats)
}

// applyWindowsStats copies a Windows reading into stats.
func applyWindowsStats(windows *windowsStats, stats *msg.MachineStats) {
	stats.Hostname = windows.Hostname
	stats.OperatingSystem = windows.Caption
	stats.KernelVersion = windows.Version
	if bootedAt, err := time.Parse(time.RFC3339Nano, windows.BootedAt); err != nil {
		recordError(stats, "uptime", fmt.Errorf("parse boot time %q: %w", windows.BootedAt, err))
	} else {
		stats.BootedAt = bootedAt.UTC().Truncate(time.Second)
		stats.UptimeSeconds = int64(stats.CollectedAt.Sub(bootedAt).Seconds())
	}

	var loadSum float64
	var loadCount int
	for _, processor := range windows.Processors {
		if stats.CPUModel == "" {
			stats.CPUModel = processor.Name
		}
		stats.CPULogicalCores += processor.LogicalProcessors
		if processor.LoadPercentage != nil {
			loadSum += *processor.LoadPercentage
			loadCount++
		}
	}
	if loadCount > 0 {
		stats.CPUUsagePercent = float64Pointer(loadSum / float64(loadCount))
	} else {
		recordError(stats, "cpu usage", fmt.Errorf("Windows reported no processor load"))
	}

	stats.MemoryTotalBytes = windows.TotalVisibleMemoryKilobytes * 1024
	stats.MemoryAvailableBytes = windows.FreePhysicalMemoryKilobytes * 1024
	stats.MemoryUsedBytes = stats.MemoryTotalBytes - stats.MemoryAvailableBytes
	for _, pageFile := range windows.PageFiles {
		stats.SwapTotalBytes += pageFile.AllocatedMegabytes * 1024 * 1024
		stats.SwapUsedBytes += pageFile.UsedMegabytes * 1024 * 1024
	}

	for _, disk := range windows.Disks {
		stats.Drives = append(stats.Drives, msg.MachineDrive{
			MountPoint:     disk.DeviceID,
			Label:          disk.VolumeName,
			FilesystemType: disk.FileSystem,
			TotalBytes:     disk.Size,
			UsedBytes:      disk.Size - disk.FreeSpace,
			FreeBytes:      disk.FreeSpace,
		})
	}

	for index, windowsBattery := range windows.Batteries {
		if windowsBattery.ChargePercent == nil {
			recordError(stats, "battery "+windowsBattery.DeviceID, fmt.Errorf("no charge reported"))
			continue
		}
		battery := msg.MachineBattery{
			Name:          strings.TrimSpace(windowsBattery.Name),
			ChargePercent: float64(*windowsBattery.ChargePercent),
			State:         windowsBatteryState(windowsBattery.Status),
		}
		if battery.Name == "" {
			battery.Name = windowsBattery.DeviceID
		}
		if minutes := windowsBattery.RunTimeMinutes; minutes != nil && *minutes > 0 && *minutes != windowsRunTimeUnknown && battery.State == msg.MachineBatteryDischarging {
			battery.SecondsRemaining = int64Pointer(*minutes * 60)
		}
		if index < len(windows.FullChargedCapacities) && index < len(windows.DesignedCapacities) && windows.DesignedCapacities[index] > 0 {
			battery.HealthPercent = float64Pointer(100 * float64(windows.FullChargedCapacities[index]) / float64(windows.DesignedCapacities[index]))
		}
		if index < len(windows.CycleCounts) && windows.CycleCounts[index] > 0 {
			battery.CycleCount = int64Pointer(windows.CycleCounts[index])
		}
		stats.Batteries = append(stats.Batteries, battery)
	}
}

// windowsBatteryState reads Win32_Battery.BatteryStatus.
func windowsBatteryState(status *int64) msg.MachineBatteryState {
	if status == nil {
		return msg.MachineBatteryUnknown
	}
	switch *status {
	case 1, 4, 5: // discharging; low; critical
		return msg.MachineBatteryDischarging
	case 2: // on mains power, not charging
		return msg.MachineBatteryNotCharging
	case 3: // fully charged
		return msg.MachineBatteryFull
	case 6, 7, 8, 9: // charging; charging and high, low or critical
		return msg.MachineBatteryCharging
	default:
		return msg.MachineBatteryUnknown
	}
}
