package block_cache_new

type block struct {
	idx            int      // Block Index
	id             string   // Block Id
	buf            *Buffer  // Inmemory buffer if exists.
	downloadStatus chan int // This channel gets handy when multiple handles works on same block at a time.
}

type blockList []block

func getBlockIndex(offset int64) int {
	return int(offset / int64(BlockSize))
}

func getBlockOffset(offset int64) int64 {
	return offset - int64(getBlockIndex(offset))*int64(BlockSize)
}
