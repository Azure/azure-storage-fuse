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
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

type BlobfuseStats struct {
	name         string
	pollInterval int
	transferPipe string
	pollingPipe  string
}

func (bfs *BlobfuseStats) GetName() string {
	return bfs.name
}

func (bfs *BlobfuseStats) SetName(name string) {
	bfs.name = name
}

func (bfs *BlobfuseStats) Monitor() error {
	defer hmcommon.Wg.Done()

	err := bfs.Validate()
	if err != nil {
		log.Err("StatsReader::Monitor : [%v]", err)
		return err
	}

	go bfs.statsPoll()

	return bfs.statsReader()
}

func (bfs *BlobfuseStats) ExportStats() {
	fmt.Println("Inside blobfuse export stats")
}

func (bfs *BlobfuseStats) Validate() error {
	if bfs.pollInterval == 0 {
		return fmt.Errorf("blobfuse-poll-interval should be non-zero")
	}

	return nil
}

func (bfs *BlobfuseStats) statsReader() error {
	err := createPipe(bfs.transferPipe)
	if err != nil {
		log.Err("StatsReader::statsReader : [%v]", err)
		return err
	}

	f, err := os.OpenFile(bfs.transferPipe, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		log.Err("StatsReader::statsReader : unable to open pipe file [%v]", err)
		return err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	var e error = nil

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			log.Err("StatsReader::statsReader : [%v]", err)
			e = err
			break
		}

		// TODO: export stats read
		fmt.Printf("Line: %v\n", string(line))
		log.Debug("StatsReader::statsReader : Line: %v\n", string(line))

		st := internal.Stats{}
		json.Unmarshal(line, &st)
		log.Debug("StatsReader::statsReader : %v, %v, %v, %v, %v\n", st.Timestamp, st.ComponentName, st.Operation, st.Path, st.Value)
	}

	return e
}

func (bfs *BlobfuseStats) statsPoll() {
	err := createPipe(bfs.pollingPipe)
	if err != nil {
		log.Err("StatsReader::statsPoll : [%v]", err)
		return
	}

	pf, err := os.OpenFile(bfs.pollingPipe, os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		log.Err("StatsManager::statsPoll : unable to open pipe file [%v]", err)
		return
	}
	defer pf.Close()

	ticker := time.NewTicker(time.Duration(bfs.pollInterval) * time.Second)
	defer ticker.Stop()

	for t := range ticker.C {
		_, err = pf.WriteString(fmt.Sprintf("Poll at %v\n", t.Format(time.RFC3339)))
		if err != nil {
			log.Err("StatsManager::statsPoll : [%v]", err)
			break
		}
	}
}

func createPipe(pipe string) error {
	_, err := os.Stat(pipe)
	if os.IsNotExist(err) {
		err = syscall.Mkfifo(pipe, 0666)
		if err != nil {
			log.Err("StatsReader::createPipe : unable to create pipe [%v]", err)
			return err
		}
	} else if err != nil {
		log.Err("StatsReader::createPipe : [%v]", err)
		return err
	}
	return nil
}

func NewBlobfuseStatsMonitor() hminternal.Monitor {
	bfs := &BlobfuseStats{
		pollInterval: hmcommon.BfsPollInterval,
		transferPipe: common.TransferPipe,
		pollingPipe:  common.PollingPipe,
	}

	bfs.SetName(hmcommon.Blobfuse_stats)

	return bfs
}

func init() {
	hminternal.AddMonitor(hmcommon.Blobfuse_stats, NewBlobfuseStatsMonitor)
}
