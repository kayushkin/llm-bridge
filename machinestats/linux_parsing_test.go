//go:build linux

package machinestats

import (
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
