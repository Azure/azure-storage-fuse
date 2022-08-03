package cpu_profiler

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	hmcommon "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/internal"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

type CpuProfiler struct {
	name         string
	pid          string
	pollInterval int
}

func (cpu *CpuProfiler) GetName() string {
	return cpu.name
}

func (cpu *CpuProfiler) SetName(name string) {
	cpu.name = name
}

func (cpu *CpuProfiler) Monitor() error {
	defer hmcommon.Wg.Done()

	err := cpu.Validate()
	if err != nil {
		log.Err("cpu_monitor::Monitor : [%v]", err)
		return err
	}

	ticker := time.NewTicker(time.Duration(cpu.pollInterval) * time.Second)
	defer ticker.Stop()

	for t := range ticker.C {
		c, err := cpu.getCpuUsage()
		if err != nil {
			log.Err("cpu_monitor::Monitor : [%v]", err)
			return err
		}

		// TODO: export cpu usage
		log.Debug("CPU Usage : %v at %v", c, t.Format(time.RFC3339))
	}

	return nil
}

func (cpu *CpuProfiler) ExportStats() {
	fmt.Println("Inside CPU export stats")
}

func (cpu *CpuProfiler) Validate() error {
	if len(cpu.pid) == 0 {
		return fmt.Errorf("pid of blobfuse2 is not given")
	}

	if cpu.pollInterval == 0 {
		return fmt.Errorf("stats-poll-interval should be non-zero")
	}

	return nil
}

func (cpu *CpuProfiler) getCpuUsage() (string, error) {
	topCmd := "top -b -n 1 -d 0.2 -p " + cpu.pid + " | tail -1 | awk '{print $9}'"

	cliOut, err := exec.Command("bash", "-c", topCmd).Output()
	if err != nil {
		log.Err("cpu_monitor::getCpuUsage : Blobfuse2 is not running on pid %v [%v]", cpu.pid, err)
		return "", err
	}

	stats := strings.Split(strings.Split(string(cliOut), "\n")[0], " ")

	if stats[0] == "%CPU" {
		log.Err("cpu_monitor::getCpuUsage : Blobfuse2 is not running on pid %v", cpu.pid)
		return "", fmt.Errorf("blobfuse2 is not running on pid %v", cpu.pid)
	}

	return stats[0], nil
}

func NewCpuMonitor() hminternal.Monitor {
	cpu := &CpuProfiler{
		pid:          hmcommon.Pid,
		pollInterval: hmcommon.StatsPollinterval,
	}

	cpu.SetName(hmcommon.Cpu_profiler)

	return cpu
}

func init() {
	hminternal.AddMonitor(hmcommon.Cpu_profiler, NewCpuMonitor)
}
