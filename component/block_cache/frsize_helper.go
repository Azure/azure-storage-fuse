// frsize_helper.go
//go:build !arm
package block_cache

func assignFrSize(val uint64) int64 {
    return int64(val)
}
