```go
// Filename: distributed_cache_test.go

import (
	"testing"
	"fmt"
)

func TestConfigure_ThriftServerType_Default(t *testing.T) {
	cache := DistributedCache{
		cfg: DistributedCacheOptions{},
	}

	err := cache.Configure(false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cache.cfg ThriftServerType != defaultThriftServerType {
		t.Errorf("expected ThriftServerType to be %s, got %s", defaultThriftServerType, cache.cfg.ThriftServerType)
	}
}

func TestConfigure_ThriftServerType_Invalid(t *testing.T) {
	cache := DistributedCache{
		cfg: DistributedCacheOptions{
			ThriftServerType: "invalid",
		},
	}

	err := cache.Configure(false)
	if err == nil {
		t.Fatal("expected an error, got none")
	}

	expectedErr := fmt.Sprintf("config error in %s: [invalid thrift-server-type (invalid), valid values are 'simple' and 'threaded']", cache.Name())
	if err.Error() != expectedErr {
		t.Errorf("expected error message to be %s, got %s", expectedErr, err.Error())
	}
}
```