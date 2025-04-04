package dcachelib

type NodeManager struct {
}

func Init(opt NodeManagerOptions, callbacks StorageCallbacks) *NodeManager {
	return &NodeManager{}
}

func (nm *NodeManager) Start() {
}

func (nm *NodeManager) Stop() {
}

func IsAlive(peerId string) bool {
	return false
}

func GetActivePeers() []Peer {
	return nil
}
