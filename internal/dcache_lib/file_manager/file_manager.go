package file_manager

type fileLayoutMgr interface {
	// Create Placeholder file to prevent the creation from the other nodes
	CreateNewFileLayout(fileName string) *fileLayout
	// Update the fileSize and its state and update the .md file.
	// The Call comes when the close call comes to write FD.
	ConfirmFileLayout(file *fileLayout, size int64) error
	// Delete Filelayout incase of write/close failure.(remove .md file)
	DeleteFileLayout(file *fileLayout) error
	// Check File is present in Dcache
	CheckFileInDCache(fileName string) (isPresent bool, f *fileLayout)
	// Check File is present in Azure.
	CheckFileInAzure(fileName string) (isPresent bool)
	// Increments File read FD count in .md file.
	IncrementFDCount(file *fileLayout) error
	// Decrements File read FD count in .md file.
	DecrementFDCount(file *fileLayout) error
}
