package distributed_cache

import (
	"encoding/json"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

type HeartbeatManager struct {
	comp         internal.Component
	cachePath    string
	hbDuration   uint32
	hbPath       string
	maxCacheSize uint64
	nodeId       string
	ticker       *time.Ticker
}

func (hm *HeartbeatManager) Start() {
	hm.ticker = time.NewTicker(time.Duration(hm.hbDuration) * time.Second)
	go func() {
		for range hm.ticker.C {
			log.Info("Scheduled task triggered")
			hm.Starthb()
		}
	}()
}

func (hm *HeartbeatManager) stopScehduler() {
	if hm.ticker != nil {
		hm.ticker.Stop()
		hm.ticker = nil
	}
}

func (hm *HeartbeatManager) Starthb() error {
	uuidVal, err := common.GetUUID()
	if err != nil {
		log.Err("AddHeartBeat: Failed to retrieve UUID, error: %v", err)
		return err
	}
	hm.nodeId = uuidVal

	hbPath := hm.hbPath + "/Nodes/" + hm.nodeId + ".hb"
	ipaddr, err := getVmIp()
	if err != nil {
		log.Err("AddHeartBeat: Failed to get VM IP")
		return err
	}
	totalSpace, used_space, err := evaluateVMStorage(hm.cachePath)
	if err != nil {
		log.Err("AddHeartBeat: Failed to evaluate VM storage: ", err)
		return err
	}
	hostname, _ := common.GetHostName()
	totalSpace = func() uint64 {
		if hm.maxCacheSize != 0 {
			return hm.maxCacheSize
		}
		return totalSpace
	}()
	hbData := map[string]interface{}{
		"ipaddr":           ipaddr,
		"nodeid":           hm.nodeId,
		"hostname":         hostname,
		"last_heartbeat":   time.Now().Unix(),
		"total_space_byte": totalSpace,
		"used_space_byte":  used_space,
	}

	// Marshal the data into JSON
	data, err := json.MarshalIndent(hbData, "", "  ")
	if err != nil {
		log.Err("AddHeartBeat: Failed to marshal heartbeat data")
		return err
	}

	// Create a heartbeat file in storage with <nodeId>.hb
	if err := hm.comp.NextComponent().WriteFromBuffer(internal.WriteFromBufferOptions{Name: hbPath, Data: data}); err != nil {
		log.Err("AddHeartBeat: Failed to write heartbeat file: ", err)
		return err
	}
	log.Info("AddHeartBeat: Heartbeat file updated successfully")
	return nil
}

func (hm *HeartbeatManager) Stop() {
	hm.stopScehduler()
	hbPath := hm.hbPath + "/Nodes/" + hm.nodeId + ".hb"
	hm.comp.NextComponent().DeleteFile(internal.DeleteFileOptions{Name: hbPath})
}
