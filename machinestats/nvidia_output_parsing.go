package machinestats

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kayushkin/llm-bridge/msg"
)

// parseNVIDIAGPUs reads nvidia-smi's
// --query-gpu=name,utilization.gpu,memory.used,memory.total,temperature.gpu
// --format=csv,noheader,nounits: one line per GPU, memory in MiB. A field
// the driver does not support reads "[N/A]" and is left nil.
func parseNVIDIAGPUs(output string) ([]msg.MachineGPU, error) {
	var gpus []msg.MachineGPU
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) != 5 {
			return nil, fmt.Errorf("nvidia-smi line %q has %d fields, want 5", line, len(fields))
		}
		for index := range fields {
			fields[index] = strings.TrimSpace(fields[index])
		}
		number := func(field string) *float64 {
			value, err := strconv.ParseFloat(field, 64)
			if err != nil {
				return nil
			}
			return &value
		}
		mebibytes := func(field string) *uint64 {
			value := number(field)
			if value == nil {
				return nil
			}
			bytes := uint64(*value * (1 << 20))
			return &bytes
		}
		gpus = append(gpus, msg.MachineGPU{
			Name:               fields[0],
			UsagePercent:       number(fields[1]),
			MemoryUsedBytes:    mebibytes(fields[2]),
			MemoryTotalBytes:   mebibytes(fields[3]),
			TemperatureCelsius: number(fields[4]),
		})
	}
	return gpus, nil
}
