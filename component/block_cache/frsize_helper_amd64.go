// frsize_helper_amd64.go
// go:build amd64 && linux
package block_cache

func assignFrSize(val uint64) int64 {
    return int64(val)
}
