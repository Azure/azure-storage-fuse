```go
// Filename: config_test.go

import (
	"testing"
	"reflect"
)

func TestThriftServerType(t *testing.T) {
	expected := ""
	if ThriftServerType != expected {
		t.Errorf("Expected ThriftServerType to be %v, got %v", expected, ThriftServerType)
	}
}
```