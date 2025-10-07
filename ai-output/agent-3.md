```go
// Filename: cluster_manager_test.go

func TestClusterManagerStartWithSimpleRPC(t *testing.T) {
	cm := &ClusterManager{
		ThriftServerType: "simple",
	}
	rvsMap := make(map[string]string) // Replace with actual initialization if necessary

	err := cm.start(&dcache.DCacheConfig{}, rvsMap)

	assert.NoError(t, err)
	assert.NotNil(t, cm.rpcServerSimple)
	assert.Nil(t, cm.rpcServerThreaded)
}

func TestClusterManagerStartWithThreadedRPC(t *testing.T) {
	cm := &ClusterManager{
		ThriftServerType: "threaded",
	}
	rvsMap := make(map[string]string) // Replace with actual initialization if necessary

	err := cm.start(&dcache.DCacheConfig{}, rvsMap)

	assert.NoError(t, err)
	assert.Nil(t, cm.rpcServerSimple)
	assert.NotNil(t, cm.rpcServerThreaded)
}

func TestClusterManagerStartWithInvalidRPC(t *testing.T) {
	cm := &ClusterManager{
		ThriftServerType: "invalid",
	}
	rvsMap := make(map[string]string) // Replace with actual initialization if necessary

	assert.Panics(t, func() {
		cm.start(&dcache.DCacheConfig{}, rvsMap)
	})
}
```