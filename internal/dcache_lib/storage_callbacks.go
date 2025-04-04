package dcachelib

type StorageCallbacks interface {
	GetBlob(blobName string) (string, error)
	PutBlob(blobName string, data string) error
	GetProperties(blobName string) (map[string]string, error)
	SetProperties(blobName string, properties map[string]string) error
	ListAllBlobs(path string) ([]string, error)
}
