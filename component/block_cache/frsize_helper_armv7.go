// frsize_helper_armv7.go
//go:build arm
package block_cache

func assignFrSize(val uint64) int32 {
    return int32(val)
}
