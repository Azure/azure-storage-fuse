```go
// Filename: dcache_file_test.go

func TestGetFileSize(t *testing.T) {
	file := &DcacheFile{
		FileMetadata: FileMetadata{Size: 100},
		CacheWarmup:  nil,
	}

	size := file.getFileSize()
	assert.Equal(t, int64(100), size)

	file.CacheWarmup = &cacheWarmup{Size: 200}
	file.FileMetadata.Size = -1

	size = file.getFileSize()
	assert.Equal(t, int64(200), size)
}

func TestGetModifiedReadaheadOnWarmup(t *testing.T) {
	cacheWarmup := &cacheWarmup{
		MaxChunks: 5,
		Bitmap:    make([]uint64, 1),
	}

	file := &DcacheFile{
		CacheWarmup: cacheWarmup,
	}

	startIdx, endIdx := file.getModifiedReadaheadOnWarmup(0, 3)
	assert.Equal(t, int64(0), startIdx)
	assert.Equal(t, int64(3), endIdx)

	// Simulate a scenario where read-ahead hits a chunk not uploaded yet
	common.AtomicSetBitUint64(&file.CacheWarmup.Bitmap[0], 0)
	common.AtomicSetBitUint64(&file.CacheWarmup.Bitmap[0], 1)

	startIdx, endIdx = file.getModifiedReadaheadOnWarmup(0, 5)
	assert.Equal(t, int64(0), startIdx)
	assert.Equal(t, int64(2), endIdx)
}
```