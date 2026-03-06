package main

import (
	"anytls/internal/ppanel"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

func collectServerStatus(ctx context.Context) (ppanel.ServerStatusRequest, error) {
	cpuPercent, err := cpu.PercentWithContext(ctx, time.Second, false)
	if err != nil {
		return ppanel.ServerStatusRequest{}, fmt.Errorf("read cpu usage: %w", err)
	}
	virtualMemory, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return ppanel.ServerStatusRequest{}, fmt.Errorf("read memory usage: %w", err)
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return ppanel.ServerStatusRequest{}, fmt.Errorf("get working dir: %w", err)
	}
	diskUsage, err := disk.UsageWithContext(ctx, workingDir)
	if err != nil {
		return ppanel.ServerStatusRequest{}, fmt.Errorf("read disk usage: %w", err)
	}
	status := ppanel.ServerStatusRequest{
		Mem:       virtualMemory.UsedPercent,
		Disk:      diskUsage.UsedPercent,
		UpdatedAt: time.Now().Unix(),
	}
	if len(cpuPercent) > 0 {
		status.CPU = cpuPercent[0]
	}
	return status, nil
}
