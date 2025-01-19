package block_cache_new

import (
	"os"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/component/azstorage"
)

func setupPipeline() {
	bc := NewBlockCacheComponent()
	az := azstorage.NewazstorageComponent()
	bc.SetNextComponent(az)

}

func TestMain(m *testing.M) {
	setupPipeline()
	os.Exit(m.Run())
}
