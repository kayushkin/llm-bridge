package msg

import "time"

// ──────────────────────────────────────────────────────────────────────────────
// Machine stats
//
// A runner reads its machine's health every minute — CPU, memory, drives,
// batteries, uptime — and sends it to the bridge as a RunnerMsgMachineStats
// frame. The bridge reads its own machine the same way, keeps every sample for
// a while, and serves the latest and the history per machine. The reading
// itself is package machinestats; these are the shapes that travel.
// ──────────────────────────────────────────────────────────────────────────────

// MachineStatsSource says where a sample's numbers come from, because a runner
// in WSL reads Windows rather than the Linux it runs in.
type MachineStatsSource string

const (
	MachineStatsSourceLinux          MachineStatsSource = "linux"
	MachineStatsSourceDarwin         MachineStatsSource = "darwin"
	MachineStatsSourceWindowsFromWSL MachineStatsSource = "windows_from_wsl"
)

// MachineStats is one reading of a machine's health. A field the machine
// could not report is left empty (nil pointer, empty list), never guessed;
// what went wrong is in CollectionErrors.
type MachineStats struct {
	CollectedAt time.Time          `json:"collected_at"`
	Source      MachineStatsSource `json:"source"`

	Hostname        string     `json:"hostname,omitempty"`
	OperatingSystem string     `json:"operating_system,omitempty"` // "Ubuntu 24.04.1 LTS", "macOS 15.1", "Microsoft Windows 11 Pro"
	KernelVersion   string     `json:"kernel_version,omitempty"`
	BootedAt        *time.Time `json:"booted_at,omitempty"`
	UptimeSeconds   int64      `json:"uptime_seconds,omitempty"`

	CPUModel        string `json:"cpu_model,omitempty"`
	CPULogicalCores int    `json:"cpu_logical_cores,omitempty"`
	// CPUUsagePercent is busy time across all cores, 0–100. On Linux it is the
	// average since the previous sample; on macOS and Windows a one-second look.
	CPUUsagePercent *float64 `json:"cpu_usage_percent,omitempty"`
	// LoadAverage is the 1, 5 and 15 minute load; Windows has none.
	LoadAverage []float64 `json:"load_average,omitempty"`

	MemoryTotalBytes     uint64 `json:"memory_total_bytes,omitempty"`
	MemoryUsedBytes      uint64 `json:"memory_used_bytes,omitempty"`
	MemoryAvailableBytes uint64 `json:"memory_available_bytes,omitempty"`
	SwapTotalBytes       uint64 `json:"swap_total_bytes,omitempty"`
	SwapUsedBytes        uint64 `json:"swap_used_bytes,omitempty"`
	// MemoryCachedBytes is file cache the OS hands back on demand; it is part
	// of MemoryAvailableBytes, not of MemoryUsedBytes.
	MemoryCachedBytes uint64 `json:"memory_cached_bytes,omitempty"`
	// MemoryCompressedBytes is RAM the compressor holds (macOS), part of
	// MemoryUsedBytes.
	MemoryCompressedBytes uint64 `json:"memory_compressed_bytes,omitempty"`
	// MemoryCommittedBytes and MemoryCommitLimitBytes are Windows' commit
	// charge: memory promised to programs, against RAM plus the page file.
	// Near the limit, programs fail to get memory.
	MemoryCommittedBytes   uint64 `json:"memory_committed_bytes,omitempty"`
	MemoryCommitLimitBytes uint64 `json:"memory_commit_limit_bytes,omitempty"`
	// MemoryFreeUnderPressurePercent is macOS' own measure of how much memory
	// it could free (kern.memorystatus_level): low means real pressure, even
	// when nearly all memory reads as used.
	MemoryFreeUnderPressurePercent *float64 `json:"memory_free_under_pressure_percent,omitempty"`

	Drives    []MachineDrive   `json:"drives,omitempty"`
	Batteries []MachineBattery `json:"batteries,omitempty"`
	GPUs      []MachineGPU     `json:"gpus,omitempty"`

	// WSL is a reading of the WSL Linux virtual machine a runner in WSL runs
	// in, beside the Windows reading this one holds: its own OS and kernel,
	// CPU, memory limit, swap, uptime and virtual disk. Nil elsewhere.
	WSL *MachineStats `json:"wsl,omitempty"`

	// CollectionErrors names each part that could not be read, so a gap in the
	// numbers is explained rather than silent.
	CollectionErrors []string `json:"collection_errors,omitempty"`
}

// MachineDrive is one mounted file system.
type MachineDrive struct {
	MountPoint     string `json:"mount_point"` // "/", "/mnt/media", "C:"
	Device         string `json:"device,omitempty"`
	Label          string `json:"label,omitempty"` // volume name, when the OS has one
	FilesystemType string `json:"filesystem_type,omitempty"`
	TotalBytes     uint64 `json:"total_bytes"`
	UsedBytes      uint64 `json:"used_bytes"`
	FreeBytes      uint64 `json:"free_bytes"` // free to an ordinary user
}

// MachineBatteryState is what a battery is doing.
type MachineBatteryState string

const (
	MachineBatteryCharging    MachineBatteryState = "charging"
	MachineBatteryDischarging MachineBatteryState = "discharging"
	MachineBatteryFull        MachineBatteryState = "full"
	MachineBatteryNotCharging MachineBatteryState = "not_charging" // on mains, holding its charge
	MachineBatteryUnknown     MachineBatteryState = "unknown"
)

// MachineBattery is one battery.
type MachineBattery struct {
	Name          string              `json:"name"`
	ChargePercent float64             `json:"charge_percent"`
	State         MachineBatteryState `json:"state"`
	// SecondsRemaining is the OS's estimate of time left on battery while
	// discharging, or to full while charging; nil when it gives none.
	SecondsRemaining *int64 `json:"seconds_remaining,omitempty"`
	// HealthPercent is full-charge capacity over design capacity.
	HealthPercent *float64 `json:"health_percent,omitempty"`
	CycleCount    *int64   `json:"cycle_count,omitempty"`
}

// MachineGPU is one graphics processor. A field the driver does not report
// is nil.
type MachineGPU struct {
	Name               string   `json:"name"`
	UsagePercent       *float64 `json:"usage_percent,omitempty"`
	MemoryUsedBytes    *uint64  `json:"memory_used_bytes,omitempty"`
	MemoryTotalBytes   *uint64  `json:"memory_total_bytes,omitempty"`
	TemperatureCelsius *float64 `json:"temperature_celsius,omitempty"`
}

// RunnerMachineStats is the payload of a RunnerMsgMachineStats frame. The
// bridge files it under the machine the runner's token belongs to, never a
// name the runner sends.
type RunnerMachineStats struct {
	Stats MachineStats `json:"stats"`
}

// MachineStatsSample is one stored reading.
type MachineStatsSample struct {
	MachineID string       `json:"machine_id"`
	Stats     MachineStats `json:"stats"`
}

// MachineStatsOverview is one machine with its newest reading, as
// GET /machines/stats returns it. Latest is nil when the machine has never
// reported. RunnerConnected is set for runner machines only.
type MachineStatsOverview struct {
	Machine         Machine       `json:"machine"`
	RunnerConnected *bool         `json:"runner_connected,omitempty"`
	Latest          *MachineStats `json:"latest,omitempty"`
}

// MachineStatsHistory is what GET /machines/{id}/stats/history returns: the
// samples in [Since, Until), grouped into buckets of StepSeconds.
type MachineStatsHistory struct {
	MachineID   string                     `json:"machine_id"`
	Since       time.Time                  `json:"since"`
	Until       time.Time                  `json:"until"`
	StepSeconds int64                      `json:"step_seconds"`
	Points      []MachineStatsHistoryPoint `json:"points"`
}

// MachineStatsHistoryPoint is one bucket of samples. Stats is the newest
// sample in the bucket; CPU is also given as the bucket's mean and peak, so a
// short spike is not lost when a long range is drawn.
type MachineStatsHistoryPoint struct {
	At                     time.Time    `json:"at"` // start of the bucket
	SampleCount            int          `json:"sample_count"`
	CPUUsagePercentAverage *float64     `json:"cpu_usage_percent_average,omitempty"`
	CPUUsagePercentMaximum *float64     `json:"cpu_usage_percent_maximum,omitempty"`
	Stats                  MachineStats `json:"stats"`
}
