```go
// Filename: node_uuid_test.go

package yourpackage

import (
	"path/filepath"
	"testing"
)

func TestGetNodeUUIDFilePath(t *testing.T) {
	expectedPath := filepath.Join(DefaultWorkDir, "blobfuse_node_uuid")
	actualPath := GetNodeUUIDFilePath()

	if expectedPath != actualPath {
		t.Errorf("Expected %s but got %s", expectedPath, actualPath)
	}
}
```