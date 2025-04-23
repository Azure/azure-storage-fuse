package replication_manager

type ReadMvRequest struct {
	FileID string
	RvID   string
	MvName string
	Offset int64
	Length int64
}

type ReadMvResponse struct {
	Data []byte
	Hash string
}

type WriteMvRequest struct {
	FileID string
	RvID   string
	MvName string
	Offset int64
	Data   []byte
	Hash   string
	Length int64
}

type WriteMvResponse struct {
	AvailableSpace int64
}

type OfflineMvRequest struct {
}

type OfflineMvResponse struct {
}
