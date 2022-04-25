package stream

type ReadCache struct {
	Stream
	StreamConnection
	blockSize           int64
	bufferSizePerHandle uint64 // maximum number of blocks allowed to be stored for a file
	handleLimit         int32
	openHandles         int32
	streamOnly          bool
}

func (r *ReadCache) Configure(conf StreamConfig) error {
	if conf.bufferSizePerHandle <= 0 || conf.blockSize <= 0 || conf.handleLimit <= 0 {
		r.streamOnly = true
	}
	r.blockSize = int64(conf.blockSize) * mb
	r.bufferSizePerHandle = conf.bufferSizePerHandle
	r.handleLimit = conf.handleLimit
	r.openHandles = 0
	return nil
}
