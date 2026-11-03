package machinestats

import (
	"math"
	"testing"

	"github.com/kayushkin/llm-bridge/msg"
)

func TestParseDarwinOutputs(t *testing.T) {
	bootedAt, err := parseDarwinBootTime("{ sec = 1762000000, usec = 5 } Sat Nov  1 12:26:40 2025")
	if err != nil || bootedAt.Unix() != 1762000000 {
		t.Fatalf("boot %v %v", bootedAt, err)
	}

	total, used, err := parseDarwinSwapUsage("total = 2048.00M  used = 1024.50M  free = 1023.50M  (encrypted)")
	if err != nil || total != 2048<<20 || used != uint64(1024.5*(1<<20)) {
		t.Fatalf("swap %d %d %v", total, used, err)
	}

	vmStat := `Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages free:                               10000.
Pages active:                            100000.
Pages inactive:                           90000.
Pages wired down:                         50000.
Pages purgeable:                           2000.
File-backed pages:                        30000.
Pages occupied by compressor:             20000.
`
	memory, err := parseDarwinMemory(vmStat)
	if err != nil || memory.used != (100000+50000+20000)*16384 || memory.compressed != 20000*16384 || memory.cached != (30000+2000)*16384 {
		t.Fatalf("memory %+v %v", memory, err)
	}
	if _, err := parseDarwinMemory("Mach Virtual Memory Statistics: (page size of 16384 bytes)\nPages active: 1.\n"); err == nil {
		t.Fatal("expected an error when vm_stat lacks a line")
	}

	diskFree := `Filesystem   1024-blocks      Used Available Capacity  Mounted on
/dev/disk3s1s1  482797652  10000000  200000000     5%    /
devfs                 210       210         0   100%    /dev
/dev/disk3s5    482797652 250000000  200000000    56%    /System/Volumes/Data
/dev/disk5s1    976000000 500000000  476000000    52%    /Volumes/Backup Drive
`
	drives, err := parseDarwinDiskFree(diskFree)
	if err != nil || len(drives) != 2 {
		t.Fatalf("drives %+v %v", drives, err)
	}
	if drives[0].MountPoint != "/" || drives[0].UsedBytes != (482797652-200000000)*1024 {
		t.Fatalf("startup drive %+v", drives[0])
	}
	if drives[1].MountPoint != "/Volumes/Backup Drive" || drives[1].Label != "Backup Drive" {
		t.Fatalf("external drive %+v", drives[1])
	}

	batteries, err := parseDarwinPowerManagementBatteries("Now drawing from 'Battery Power'\n -InternalBattery-0 (id=4653155)\t85%; discharging; 4:12 remaining present: true\n")
	if err != nil || len(batteries) != 1 {
		t.Fatalf("batteries %+v %v", batteries, err)
	}
	if battery := batteries[0]; battery.Name != "InternalBattery-0" || battery.ChargePercent != 85 || battery.State != msg.MachineBatteryDischarging || *battery.SecondsRemaining != 4*3600+12*60 {
		t.Fatalf("battery %+v", battery)
	}
	batteries, _ = parseDarwinPowerManagementBatteries(" -InternalBattery-0 (id=1)\t100%; AC attached; not charging present: true\n")
	if batteries[0].State != msg.MachineBatteryNotCharging || batteries[0].SecondsRemaining != nil {
		t.Fatalf("battery on mains %+v", batteries[0])
	}
	batteries, _ = parseDarwinPowerManagementBatteries(" -InternalBattery-0 (id=1)\t40%; charging; (no estimate) present: true\n")
	if batteries[0].State != msg.MachineBatteryCharging || batteries[0].SecondsRemaining != nil {
		t.Fatalf("battery charging %+v", batteries[0])
	}

	health, cycles := parseDarwinBatteryRegistry(`  "MaxCapacity" = 100
  "AppleRawMaxCapacity" = 4500
  "DesignCapacity" = 5000
  "CycleCount" = 321`)
	if health == nil || math.Abs(*health-90) > 0.001 || cycles == nil || *cycles != 321 {
		t.Fatalf("health %v cycles %v", health, cycles)
	}
}

// TestParseDarwinOutputsFromRealMachine feeds output an Apple silicon laptop
// on macOS 15 printed.
func TestParseDarwinOutputsFromRealMachine(t *testing.T) {
	diskFree := `Filesystem     1024-blocks     Used Available Capacity  Mounted on
/dev/disk3s3s1   239362496 10998572 199124480     6%    /
/dev/disk3s6     239362496       20 199124480     1%    /System/Volumes/VM
/dev/disk3s4     239362496  6934744 199124480     4%    /System/Volumes/Preboot
/dev/disk1s2        512000     6164    493232     2%    /System/Volumes/xarts
/dev/disk3s1     239362496 21105640 199124480    10%    /System/Volumes/Data
`
	drives, err := parseDarwinDiskFree(diskFree)
	if err != nil || len(drives) != 1 || drives[0].MountPoint != "/" || drives[0].UsedBytes != (239362496-199124480)*1024 {
		t.Fatalf("drives %+v %v", drives, err)
	}
	batteries, err := parseDarwinPowerManagementBatteries("Now drawing from 'Battery Power'\n -InternalBattery-0 (id=6357091)\t9%; discharging; 7:15 remaining present: true\n\tBattery Warning: Early\n")
	if err != nil || len(batteries) != 1 || batteries[0].ChargePercent != 9 || *batteries[0].SecondsRemaining != 7*3600+15*60 {
		t.Fatalf("batteries %+v %v", batteries, err)
	}
	health, cycles := parseDarwinBatteryRegistry("      \"MaxCapacity\" = 100\n      \"DesignCapacity\" = 4563\n      \"CycleCount\" = 30\n      \"AppleRawMaxCapacity\" = 4460\n")
	if health == nil || math.Abs(*health-97.74) > 0.01 || *cycles != 30 {
		t.Fatalf("health %v cycles %v", health, cycles)
	}
	total, used, err := parseDarwinSwapUsage("total = 0.00M  used = 0.00M  free = 0.00M  (encrypted)")
	if err != nil || total != 0 || used != 0 {
		t.Fatalf("swap %d %d %v", total, used, err)
	}
	if matches := darwinCPUIdlePattern.FindAllStringSubmatch("CPU usage: 7.25% user, 13.70% sys, 79.3% idle \nCPU usage: 4.67% user, 3.47% sys, 91.84% idle \n", -1); len(matches) != 2 || matches[1][1] != "91.84" {
		t.Fatalf("cpu %v", matches)
	}
	bootedAt, err := parseDarwinBootTime("{ sec = 1792523986, usec = 438043 } Tue Oct 20 12:19:46 2026")
	if err != nil || bootedAt.Unix() != 1792523986 {
		t.Fatalf("boot %v %v", bootedAt, err)
	}
}

func TestParseDarwinGPUsFromRealMachine(t *testing.T) {
	ioreg := `      "PerformanceStatistics" = {"In use system memory (driver)"=0,"Alloc system memory"=1095106560,"Tiler Utilization %"=18,"Renderer Utilization %"=17,"Device Utilization %"=18,"In use system memory"=219185152}
      "model" = "Apple M2"`
	gpus := parseDarwinGPUs(ioreg)
	if len(gpus) != 1 || gpus[0].Name != "Apple M2" || *gpus[0].UsagePercent != 18 || *gpus[0].MemoryUsedBytes != 219185152 || gpus[0].MemoryTotalBytes != nil {
		t.Fatalf("gpus %+v", gpus)
	}
	if len(parseDarwinGPUs("")) != 0 {
		t.Fatal("no accelerator, no GPU")
	}
}

func TestParseNVIDIAGPUs(t *testing.T) {
	gpus, err := parseNVIDIAGPUs("NVIDIA GeForce RTX 3080 Ti, 0, 3484, 12288, 31\nExample GPU, [N/A], 10, 100, [N/A]\n")
	if err != nil || len(gpus) != 2 {
		t.Fatalf("gpus %+v %v", gpus, err)
	}
	first := gpus[0]
	if first.Name != "NVIDIA GeForce RTX 3080 Ti" || *first.UsagePercent != 0 || *first.MemoryUsedBytes != 3484<<20 || *first.MemoryTotalBytes != 12288<<20 || *first.TemperatureCelsius != 31 {
		t.Fatalf("first %+v", first)
	}
	if gpus[1].UsagePercent != nil || gpus[1].TemperatureCelsius != nil {
		t.Fatalf("[N/A] must be nil: %+v", gpus[1])
	}
	if _, err := parseNVIDIAGPUs("only, three, fields"); err == nil {
		t.Fatal("expected an error for a short line")
	}
}
