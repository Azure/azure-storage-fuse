package memory_profiler

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	hmcommon "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/internal"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

type MemoryProfiler struct {
	name         string
	pid          string
	pollInterval int
}

func (mem *MemoryProfiler) GetName() string {
	return mem.name
}

func (mem *MemoryProfiler) SetName(name string) {
	mem.name = name
}

func (mem *MemoryProfiler) Monitor() error {
	defer hmcommon.Wg.Done()

	err := mem.Validate()
	if err != nil {
		log.Err("memory_monitor::Monitor : [%v]", err)
		return err
	}

	ticker := time.NewTicker(time.Duration(mem.pollInterval) * time.Second)
	defer ticker.Stop()

	for t := range ticker.C {
		c, err := mem.getMemoryUsage()
		if err != nil {
			log.Err("memory_monitor::Monitor : [%v]", err)
			return err
		}

		// TODO: export memory usage
		log.Debug("Memory Usage : %v at %v", c, t.Format(time.RFC3339))
	}

	return nil
}

func (mem *MemoryProfiler) ExportStats() {
	fmt.Println("Inside memory export stats")
}

func (mem *MemoryProfiler) Validate() error {
	if len(mem.pid) == 0 {
		return fmt.Errorf("pid of blobfuse2 is not given")
	}

	if mem.pollInterval == 0 {
		return fmt.Errorf("stats-poll-interval should be non-zero")
	}

	return nil
}

func (mem *MemoryProfiler) getMemoryUsage() (string, error) {
	topCmd := "top -b -n 1 -d 0.2 -p " + mem.pid + " | tail -1 | awk '{print $10}'"

	cliOut, err := exec.Command("bash", "-c", topCmd).Output()
	if err != nil {
		log.Err("memory_monitor::getMemoryUsage : Blobfuse2 is not running on pid %v [%v]", mem.pid, err)
		return "", err
	}

	stats := strings.Split(strings.Split(string(cliOut), "\n")[0], " ")

	if stats[0] == "%MEM" {
		log.Err("memory_monitor::getMemoryUsage : Blobfuse2 is not running on pid %v", mem.pid)
		return "", fmt.Errorf("blobfuse2 is not running on pid %v", mem.pid)
	}

	return stats[0], nil
}

func NewMemoryMonitor() hminternal.Monitor {
	mem := &MemoryProfiler{
		pid:          hmcommon.Pid,
		pollInterval: hmcommon.StatsPollinterval,
	}

	mem.SetName(hmcommon.Memory_profiler)

	return mem
}

func init() {
	hminternal.AddMonitor(hmcommon.Memory_profiler, NewMemoryMonitor)
}
