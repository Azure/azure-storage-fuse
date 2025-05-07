package debug

// The functions that were implemented inside this file should have Callback as the suffix for their functionName.
// The function should have this decl func(*procFile) error.

// proc file: clusterMap.json
func readClusterMapCallback(pFile *procFile) error {
	pFile.buf = []byte("Hello, World!!")
	return nil
}
