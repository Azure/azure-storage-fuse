// frsize_helper_arm.go
// go:build arm && linux
package block_cache

func assignFrSize(val uint64) int32 {
    return int32(val)
}
