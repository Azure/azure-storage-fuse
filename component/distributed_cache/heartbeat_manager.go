package distributed_cache

import (
	"encoding/json"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/azstorage"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

type HeartbeatManager struct {
	cachePath  string
	storage    azstorage.AzConnection
	hbDuration uint32
	nodeId     string
	ticker     *time.Ticker
	hbPath     string
}

func NewHeartbeatManager(cachePath string, storage azstorage.AzConnection, hbDuration uint32, hbPath string) *HeartbeatManager {
	return &HeartbeatManager{
		cachePath:  cachePath,
		hbPath:     hbPath,
		storage:    storage,
		hbDuration: hbDuration,
	}
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
	total, free, err := evaluateVMStorage(hm.cachePath)
	if err != nil {
		log.Err("AddHeartBeat: Failed to evaluate VM storage: ", err)
		return err
	}
	hostname, _ := common.GetHostName()
	hbData := map[string]interface{}{
		"ipaddr":             ipaddr,
		"nodeid":             hm.nodeId,
		"hostname":           hostname,
		"last_heartbeat":     time.Now().Unix(),
		"total_space_GB":     total / (1024 * 1024 * 1024),
		"available_space_GB": free / (1024 * 1024 * 1024),
	}

	// Marshal the data into JSON
	data, err := json.MarshalIndent(hbData, "", "  ")
	if err != nil {
		log.Err("AddHeartBeat: Failed to marshal heartbeat data")
		return err
	}

	// Create a heartbeat file in storage with <nodeId>.hb
	if err := hm.storage.WriteFromBuffer(internal.WriteFromBufferOptions{Name: hbPath, Data: data}); err != nil {
		log.Err("AddHeartBeat: Failed to write heartbeat file: ", err)
		return err
	}
	log.Info("AddHeartBeat: Heartbeat file updated successfully")
	return nil
}

func (hm *HeartbeatManager) Stop() {
	hm.stopScehduler()
	hbPath := hm.hbPath + "/Nodes/" + hm.nodeId + ".hb"
	hm.storage.DeleteFile(hbPath)
}
