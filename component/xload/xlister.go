package xload

import (
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

var _ xcomponent = &xlister{}
var _ xcomponent = &localLister{}

type xlister struct {
	xbase
	path string // base path of the directory to be listed
}

type xenumerator interface {
	mkdir(name string) error
	// getInputPool() *ThreadPool
	// setOutputPool(pool *ThreadPool)
}

type localLister struct {
	xlister
}

func newLocalXLister(path string, remote internal.Component) (*localLister, error) {
	ll := &localLister{
		xlister: xlister{
			path: path,
			xbase: xbase{
				remote: remote,
			},
		},
	}
	ll.init()
	return ll, nil
}

func (ll *localLister) init() {
	ll.pool = newThreadPool(MAX_LISTER, ll.process)
	if ll.pool == nil {
		log.Err("Xlister::newLocalLister : fail to init thread pool")
	}
}

func (ll *localLister) start() {
}

func (ll *localLister) stop() {
}

func (ll *localLister) process(item *workItem) (int, error) {
	return 0, nil
}
