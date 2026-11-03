//go:build linux

package machinestats

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/kayushkin/llm-bridge/msg"
)

func TestParseLinuxCPUTimeCounters(t *testing.T) {
	counters, err := parseLinuxCPUTimeCounters("cpu  100 5 50 800 20 3 2 1 9 0\ncpu0 1 2 3 4 5 6 7 8\n")
	if err != nil {
		t.Fatal(err)
	}
	// busy = user+nice+system+irq+softirq+steal; idle and iowait excluded; guest not counted twice.
	if counters.busy != 100+5+50+3+2+1 || counters.total != 100+5+50+800+20+3+2+1 {
		t.Fatalf("got %+v", counters)
	}
	if _, err := parseLinuxCPUTimeCounters("intr 1 2\n"); err == nil {
		t.Fatal("expected an error without a cpu line")
	}
}

func TestParseLinuxCPUInformation(t *testing.T) {
	model, cores := parseLinuxCPUInformation("processor\t: 0\nmodel name\t: Example CPU\nprocessor\t: 1\nmodel name\t: Example CPU\n")
	if model != "Example CPU" || cores != 2 {
		t.Fatalf("got %q %d", model, cores)
	}
}

func TestParseLinuxMemoryInformation(t *testing.T) {
	values, err := parseLinuxMemoryInformation("MemTotal:       16000 kB\nMemAvailable:    4000 kB\nHugePages_Total:       0\n")
	if err != nil {
		t.Fatal(err)
	}
	if values["MemTotal"] != 16000*1024 || values["MemAvailable"] != 4000*1024 || values["HugePages_Total"] != 0 {
		t.Fatalf("got %v", values)
	}
}

func TestSelectLinuxDriveMounts(t *testing.T) {
	procMounts := `/dev/sda1 / ext4 rw 0 0
/dev/sda1 /var/lib/docker/bind ext4 rw 0 0
/dev/loop3 /snap/core/1 squashfs ro 0 0
tmpfs /run tmpfs rw 0 0
overlay /var/lib/docker/overlay2/x/merged overlay rw 0 0
pool/media /mnt/pool zfs rw 0 0
media:/srv /mnt/media\040share nfs4 rw 0 0
/dev/sdb1 /boot/efi vfat rw 0 0
`
	mounts := selectLinuxDriveMounts(procMounts)
	var mountPoints []string
	for _, mount := range mounts {
		mountPoints = append(mountPoints, mount.mountPoint)
	}
	want := []string{"/", "/boot/efi", "/mnt/media share", "/mnt/pool"}
	if len(mountPoints) != len(want) {
		t.Fatalf("got %q, want %q", mountPoints, want)
	}
	for index := range want {
		if mountPoints[index] != want[index] {
			t.Fatalf("got %q, want %q", mountPoints, want)
		}
	}
}

func TestApplyWindowsStats(t *testing.T) {
	twelve, forty := 12.0, 40.0
	charge, discharging, minutes := int64(64), int64(1), int64(95)
	windows := windowsStats{
		Hostname: "DESKTOP", Caption: "Microsoft Windows 11 Pro", Version: "10.0.22631",
		BootedAt:                    "2026-11-01T10:00:00.0000000Z",
		TotalVisibleMemoryKilobytes: 32 * 1024 * 1024, FreePhysicalMemoryKilobytes: 8 * 1024 * 1024,
		FullChargedCapacities: []int64{45000}, DesignedCapacities: []int64{50000}, CycleCounts: []int64{210},
	}
	windows.Processors = append(windows.Processors, struct {
		Name              string   `json:"name"`
		LogicalProcessors int      `json:"logical_processors"`
		LoadPercentage    *float64 `json:"load_percentage"`
	}{"Example CPU", 16, &twelve}, struct {
		Name              string   `json:"name"`
		LogicalProcessors int      `json:"logical_processors"`
		LoadPercentage    *float64 `json:"load_percentage"`
	}{"Example CPU", 16, &forty})
	windows.Disks = append(windows.Disks, struct {
		DeviceID   string `json:"device_id"`
		VolumeName string `json:"volume_name"`
		FileSystem string `json:"file_system"`
		Size       uint64 `json:"size"`
		FreeSpace  uint64 `json:"free_space"`
	}{"C:", "Windows", "NTFS", 1000, 250})
	windows.Batteries = append(windows.Batteries, struct {
		Name           string `json:"name"`
		DeviceID       string `json:"device_id"`
		ChargePercent  *int64 `json:"charge_percent"`
		Status         *int64 `json:"status"`
		RunTimeMinutes *int64 `json:"run_time_minutes"`
	}{"Example Battery", "1", &charge, &discharging, &minutes})

	stats := msg.MachineStats{CollectedAt: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)}
	applyWindowsStats(&windows, &stats)
	if len(stats.CollectionErrors) != 0 {
		t.Fatalf("errors: %v", stats.CollectionErrors)
	}
	if stats.UptimeSeconds != 24*3600 || stats.CPULogicalCores != 32 || *stats.CPUUsagePercent != 26 {
		t.Fatalf("uptime/cpu wrong: %+v", stats)
	}
	if stats.MemoryUsedBytes != 24*1024*1024*1024 {
		t.Fatalf("memory used %d", stats.MemoryUsedBytes)
	}
	if drive := stats.Drives[0]; drive.MountPoint != "C:" || drive.UsedBytes != 750 || drive.Label != "Windows" {
		t.Fatalf("drive %+v", drive)
	}
	battery := stats.Batteries[0]
	if battery.State != msg.MachineBatteryDischarging || *battery.SecondsRemaining != 95*60 || *battery.HealthPercent != 90 || *battery.CycleCount != 210 {
		t.Fatalf("battery %+v", battery)
	}
}

func TestWindowsRunTimeUnknownIsDropped(t *testing.T) {
	charge, onMains, unknown := int64(100), int64(2), int64(windowsRunTimeUnknown)
	windows := windowsStats{BootedAt: "2026-11-01T10:00:00Z"}
	windows.Batteries = append(windows.Batteries, struct {
		Name           string `json:"name"`
		DeviceID       string `json:"device_id"`
		ChargePercent  *int64 `json:"charge_percent"`
		Status         *int64 `json:"status"`
		RunTimeMinutes *int64 `json:"run_time_minutes"`
	}{"", "1", &charge, &onMains, &unknown})
	stats := msg.MachineStats{CollectedAt: time.Now()}
	applyWindowsStats(&windows, &stats)
	if battery := stats.Batteries[0]; battery.SecondsRemaining != nil || battery.State != msg.MachineBatteryNotCharging || battery.Name != "1" {
		t.Fatalf("battery %+v", battery)
	}
}

func TestReadLinuxBattery(t *testing.T) {
	directory := t.TempDir()
	write := func(name, value string) {
		t.Helper()
		if err := writeFile(directory+"/"+name, value); err != nil {
			t.Fatal(err)
		}
	}
	write("type", "Battery")
	write("capacity", "50")
	write("status", "Discharging")
	write("energy_now", "25000000")
	write("energy_full", "50000000")
	write("energy_full_design", "62500000")
	write("power_now", "12500000")
	write("cycle_count", "77")
	battery, err := readLinuxBattery("BAT0", directory)
	if err != nil {
		t.Fatal(err)
	}
	if battery.State != msg.MachineBatteryDischarging || *battery.SecondsRemaining != 2*3600 || *battery.HealthPercent != 80 || *battery.CycleCount != 77 {
		t.Fatalf("battery %+v", battery)
	}
}

func writeFile(path, value string) error { return os.WriteFile(path, []byte(value), 0o644) }

// TestApplyWindowsStatsFromRealOutput feeds windowsStatsScript's output as a
// Windows 11 desktop with no battery printed it (hostname changed).
func TestApplyWindowsStatsFromRealOutput(t *testing.T) {
	output := `{"hostname":"DESKTOP","caption":"Microsoft Windows 11 Pro","version":"10.0.26200","booted_at":"2026-10-14T08:34:20.7490020Z","total_visible_memory_kilobytes":33466364,"free_physical_memory_kilobytes":3857848,"processors":[{"name":"Example 8-Core Processor","logical_processors":16,"load_percentage":11}],"disks":[{"device_id":"C:","volume_name":"","file_system":"NTFS","size":3999674658816,"free_space":372711669760},{"device_id":"G:","volume_name":"Cloud Drive","file_system":"FAT32","size":2199023255552,"free_space":354076086272}],"batteries":[],"full_charged_capacities":[],"designed_capacities":[],"cycle_counts":[],"page_files":[{"allocated_megabytes":16727,"used_megabytes":2150}],"committed_bytes":47449538560,"commit_limit_bytes":51809349632,"standby_cache_bytes":3795271680}`
	var windows windowsStats
	if err := json.Unmarshal([]byte(output), &windows); err != nil {
		t.Fatal(err)
	}
	stats := msg.MachineStats{CollectedAt: time.Date(2026, 11, 3, 22, 0, 0, 0, time.UTC)}
	applyWindowsStats(&windows, &stats)
	if len(stats.CollectionErrors) != 0 {
		t.Fatalf("errors: %v", stats.CollectionErrors)
	}
	if stats.BootedAt == nil || stats.BootedAt.Format(time.RFC3339) != "2026-10-14T08:34:20Z" || *stats.CPUUsagePercent != 11 || stats.CPULogicalCores != 16 {
		t.Fatalf("stats %+v", stats)
	}
	if stats.MemoryCommittedBytes != 47449538560 || stats.MemoryCommitLimitBytes != 51809349632 || stats.MemoryCachedBytes != 3795271680 {
		t.Fatalf("memory detail %+v", stats)
	}
	if len(stats.Drives) != 2 || stats.Drives[1].Label != "Cloud Drive" || len(stats.Batteries) != 0 || stats.SwapUsedBytes != 2150<<20 {
		t.Fatalf("drives %+v batteries %+v swap %d", stats.Drives, stats.Batteries, stats.SwapUsedBytes)
	}
}

// TestSelectLinuxDriveMountsInWSL keeps only the virtual disk of a WSL
// virtual machine, from mounts a WSL 2 Ubuntu listed: Windows' drives (9p),
// WSL's own overlays, snaps and the disk's second mount points are left out.
func TestSelectLinuxDriveMountsInWSL(t *testing.T) {
	procMounts := `none /usr/lib/modules/5.15.167.4-microsoft-standard-WSL2 overlay rw 0 0
drivers /usr/lib/wsl/drivers 9p ro 0 0
/dev/sdc / ext4 rw 0 0
/dev/sdc /mnt/wslg/distro ext4 ro 0 0
none /usr/lib/wsl/lib overlay rw 0 0
rootfs /init rootfs ro 0 0
C:\134 /mnt/c 9p rw 0 0
D:\134 /mnt/d 9p rw 0 0
/dev/sdc /snap ext4 rw 0 0
snapfuse /snap/bare/5 fuse.snapfuse ro 0 0
`
	mounts := selectLinuxDriveMounts(procMounts)
	if len(mounts) != 1 || mounts[0].mountPoint != "/" || mounts[0].device != "/dev/sdc" {
		t.Fatalf("got %+v, want only / on /dev/sdc", mounts)
	}
}
