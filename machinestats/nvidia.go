//go:build linux

package machinestats

import (
	"context"
	"os"
	"os/exec"

	"github.com/kayushkin/llm-bridge/msg"
)

// wslNVIDIASystemManagementInterface is where WSL puts the Windows NVIDIA
// driver's nvidia-smi; a runner started by systemd often lacks it on PATH.
const wslNVIDIASystemManagementInterface = "/usr/lib/wsl/lib/nvidia-smi"

// readNVIDIAGPUs asks nvidia-smi for each NVIDIA GPU. A machine without
// nvidia-smi has no NVIDIA GPU to report, which is not an error.
func readNVIDIAGPUs(ctx context.Context, stats *msg.MachineStats) {
	path, err := exec.LookPath("nvidia-smi")
	if err != nil {
		if _, statErr := os.Stat(wslNVIDIASystemManagementInterface); statErr != nil {
			return
		}
		path = wslNVIDIASystemManagementInterface
	}
	output, err := runCommand(ctx, "", path, "--query-gpu=name,utilization.gpu,memory.used,memory.total,temperature.gpu", "--format=csv,noheader,nounits")
	if err != nil {
		recordError(stats, "gpu", err)
		return
	}
	gpus, err := parseNVIDIAGPUs(output)
	if err != nil {
		recordError(stats, "gpu", err)
		return
	}
	stats.GPUs = append(stats.GPUs, gpus...)
}
