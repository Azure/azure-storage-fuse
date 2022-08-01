package blobfuse_stats

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"

	hmcommon "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/internal"
)

type BlobfuseStats struct {
	name         string
	pollInterval int
	transferPipe string
	pollingPipe  string
}

type Stats struct {
	Timestamp     string                 `json:"timestamp"`
	ComponentName string                 `json:"componentName"`
	Operation     string                 `json:"operation"`
	Path          string                 `json:"path"`
	Value         map[string]interface{} `json:"value"`
}

func (bfs *BlobfuseStats) GetName() string {
	return bfs.name
}

func (bfs *BlobfuseStats) SetName(name string) {
	bfs.name = name
}

func (bfs *BlobfuseStats) Monitor() error {
	go bfs.StatsPolling()

	return bfs.StatsReader()
}

func (bfs *BlobfuseStats) ExportStats() {
	fmt.Println("Inside blobfuse export stats")
}

func (bfs *BlobfuseStats) StatsReader() error {
	err := createPipe(bfs.transferPipe)
	if err != nil {
		fmt.Printf("StatsReader::Reader : [%v]", err)
		return err
	}

	f, err := os.OpenFile(bfs.transferPipe, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		fmt.Printf("StatsReader::Reader : unable to open pipe file [%v]", err)
		return err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	var e error = nil

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			fmt.Printf("StatsReader::Reader : [%v]", err)
			e = err
			break
		}
		fmt.Printf("Line: %v\n", string(line))

		st := Stats{}
		json.Unmarshal(line, &st)
		fmt.Printf("%v : %v %v %v\n", st.ComponentName, st.Path, st.Operation, st.Value)
	}

	return e
}

func (bfs *BlobfuseStats) StatsPolling() {
	err := createPipe(bfs.pollingPipe)
	if err != nil {
		fmt.Printf("StatsReader::Polling : [%v]", err)
		return
	}

	pf, err := os.OpenFile(bfs.pollingPipe, os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		fmt.Printf("StatsManager::Polling : unable to open pipe file [%v]", err)
		return
	}
	defer pf.Close()

	ticker := time.NewTicker(time.Duration(bfs.pollInterval) * time.Second)
	defer ticker.Stop()

	for t := range ticker.C {
		_, err = pf.WriteString(fmt.Sprintf("Poll at %v\n", t.Format(time.RFC3339)))
		if err != nil {
			fmt.Printf("StatsManager::Polling : [%v]", err)
			break
		}
	}
}

func createPipe(pipe string) error {
	_, err := os.Stat(pipe)
	if os.IsNotExist(err) {
		err = syscall.Mkfifo(pipe, 0666)
		if err != nil {
			fmt.Printf("StatsReader::createPipe : unable to create pipe [%v]", err)
			return err
		}
	} else if err != nil {
		fmt.Printf("StatsReader::createPipe : [%v]", err)
		return err
	}
	return nil
}

func NewBlobfuseStatsMonitor() hminternal.Monitor {
	bfs := &BlobfuseStats{
		pollInterval: hmcommon.BfsPollInterval,
		transferPipe: hmcommon.TransferPipe,
		pollingPipe:  hmcommon.PollingPipe,
	}

	bfs.SetName(hmcommon.Blobfuse_stats)

	return bfs
}

func init() {
	fmt.Println("Inside Blobfuse stats")
	hminternal.AddMonitor(hmcommon.Blobfuse_stats, NewBlobfuseStatsMonitor)
}
