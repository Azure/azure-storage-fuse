```go
// filename: dcache_file_test.go

func TestReadPartialFile(t *testing.T) {
	// Setup
	file := &DcacheFile{
		CacheWarmup: &cacheWarmup{
			Size:      100, // example size
			warmDcFile: &mockWarmDcFile{}, // assume mockWarmDcFile implements required interface
		},
	}

	buf := make([]byte, 50) // example buffer
	offset := int64(10)

	// Test ReadPartialFile
	bytesRead, err := file.ReadPartialFile(offset, &buf)
	assert.NoError(t, err)
	assert.Equal(t, 50, bytesRead) // expect bytes read to match buffer size
}

func TestReadPartialFileBeyondEOF(t *testing.T) {
	// Setup
	file := &DcacheFile{
		CacheWarmup: &cacheWarmup{
			Size:      100,
			warmDcFile: &mockWarmDcFile{},
		},
	}

	buf := make([]byte, 50)
	offset := int64(150) // beyond eof

	// Test ReadPartialFile beyond EOF
	bytesRead, err := file.ReadPartialFile(offset, &buf)
	assert.ErrorIs(t, err, io.EOF)
	assert.Equal(t, 0, bytesRead)
}
```