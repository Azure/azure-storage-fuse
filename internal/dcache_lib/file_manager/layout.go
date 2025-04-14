package file_manager

import "errors"

type fileState int

const (
	Ready fileState = iota
	Writing
	Deleting
)

type fileLayout struct {
	fileName        string
	fileId          string // UUID to represent the file inside Dcache
	size            int64
	state           fileState
	openCount       int      // Number of Read Fd's present for this file accross the dCache
	clustermapEpoch int      // Clustermap Epoch value when the file was created.
	hash            string   // Hash of the entire file data.
	mvlist          []string // todo: this should be replaced with type of mv.
}

var _ fileLayoutMgr = &fileLayout{}

// Create Placeholder file to prevent the creation from the other nodes
func (*fileLayout) CreateNewFileLayout(fileName string) *fileLayout {
	return nil
}

// Update the fileSize and its state and update the .md file.
// The Call comes when the close call comes to write FD.
func (*fileLayout) ConfirmFileLayout(file *fileLayout, size int64) error {
	return nil
}

// Delete Filelayout incase of write/close failure.(remove .md file)
func (*fileLayout) DeleteFileLayout(file *fileLayout) error {
	return nil
}

// Check File is present in Dcache, return filelayout if it's present
func (*fileLayout) CheckFileInDCache(fileName string) (isPresent bool, f *fileLayout) {
	return false, nil
}

// Check File is present in Azure.
func (*fileLayout) CheckFileInAzure(fileName string) (isPresent bool) {
	return false
}

// Increments File read FD count in .md file.
func (*fileLayout) IncrementFDCount(file *fileLayout) error {
	return errors.New("todo: Implement with unlink/remove")
}

// Decrements File read FD count in .md file.
func (*fileLayout) DecrementFDCount(file *fileLayout) error {
	return errors.New("todo: Implement with unlink/remove")
}
