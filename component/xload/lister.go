package xload

import (
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// verify that the below types implement the xcomponent interfaces
var _ xcomponent = &lister{}
var _ xcomponent = &localLister{}
var _ xcomponent = &remoteLister{}

// verify that the below types implement the xenumerator interfaces
var _ enumerator = &localLister{}
var _ enumerator = &remoteLister{}

const LISTER string = "lister"

type lister struct {
	xbase
	path string // base path of the directory to be listed
}

type enumerator interface {
	mkdir(name string) error
}

// --------------------------------------------------------------------------------------------------------

type localLister struct {
	lister
}

func newLocalLister(path string, remote internal.Component) (*localLister, error) {
	log.Debug("lister::newLocalLister : create new local lister for %s", path)

	ll := &localLister{
		lister: lister{
			path: path,
			xbase: xbase{
				remote: remote,
			},
		},
	}

	ll.setName(LISTER)
	ll.init()
	return ll, nil
}

func (ll *localLister) init() {
	ll.pool = newThreadPool(MAX_LISTER, ll.process)
	if ll.pool == nil {
		log.Err("localLister::init : fail to init thread pool")
	}
}

func (ll *localLister) start() {
	log.Debug("localLister::start : start local lister for %s", ll.path)
	ll.getThreadPool().Start()
	ll.getThreadPool().Schedule(&workItem{compName: ll.getName()})
}

func (ll *localLister) stop() {
	log.Debug("localLister::stop : stop local lister for %s", ll.path)

	if ll.getThreadPool() != nil {
		ll.getThreadPool().Stop()
	}
	ll.getNext().stop()
}

func (ll *localLister) process(item *workItem) (int, error) {
	absPath := filepath.Join(ll.path, item.path)

	log.Debug("localLister::process : Reading local dir %s", absPath)

	entries, err := os.ReadDir(absPath)
	if err != nil {
		log.Err("localLister::process : [%s]", err.Error())
		return 0, err
	}

	for _, entry := range entries {
		relPath := filepath.Join(item.path, entry.Name())
		log.Debug("localLister::process : Iterating: %s, Is directory: %v", relPath, entry.IsDir())

		if entry.IsDir() {
			// spawn go routine for directory creation and then
			// adding to the input channel of the listing component
			go func(name string) {
				err = ll.mkdir(name)
				// TODO:: xload : handle error
				if err != nil {
					log.Err("localLister::process : Failed to create directory [%s]", err.Error())
					return
				}

				ll.getThreadPool().Schedule(&workItem{
					compName: ll.getName(),
					path:     name,
				})
			}(relPath)

		} else {
			info, err := os.Stat(filepath.Join(absPath, entry.Name()))
			if err == nil {
				// send file to the output channel for chunking
				ll.getNext().getThreadPool().Schedule(&workItem{
					compName: ll.getNext().getName(),
					path:     relPath,
					dataLen:  uint64(info.Size()),
				})
			} else {
				log.Err("localLister::process : Failed to get stat of %v", relPath)
			}
		}
	}

	return len(entries), nil
}

func (ll *localLister) mkdir(name string) error {
	// create directory in container
	return ll.getRemote().CreateDir(internal.CreateDirOptions{
		Name: name,
		Mode: 0777,
	})
}

// --------------------------------------------------------------------------------------------------------

type remoteLister struct {
	lister
	listBlocked bool
}

func newRemoteLister(path string, remote internal.Component) (*remoteLister, error) {
	log.Debug("lister::newRemoteLister : create new remote lister for %s", path)

	rl := &remoteLister{
		lister: lister{
			path: path,
			xbase: xbase{
				remote: remote,
			},
		},
		listBlocked: false,
	}

	rl.setName(LISTER)
	rl.init()
	return rl, nil
}

func (rl *remoteLister) init() {
	rl.pool = newThreadPool(MAX_LISTER, rl.process)
	if rl.pool == nil {
		log.Err("remoteLister::init : fail to init thread pool")
	}
}

func (rl *remoteLister) start() {
	log.Debug("remoteLister::start : start remote lister for %s", rl.path)
	rl.getThreadPool().Start()
	rl.getThreadPool().Schedule(&workItem{compName: rl.getName()})
}

func (rl *remoteLister) stop() {
	log.Debug("remoteLister::stop : stop remote lister for %s", rl.path)
	if rl.getThreadPool() != nil {
		rl.getThreadPool().Stop()
	}
	rl.getNext().stop()
}

// wait for the configured block-list-on-mount-sec to make the list call
func waitForListTimeout() error {
	var blockListSeconds uint16 = 0
	err := config.UnmarshalKey("azstorage.block-list-on-mount-sec", &blockListSeconds)
	if err != nil {
		return err
	}
	time.Sleep(time.Duration(blockListSeconds) * time.Second)
	return nil
}

func (rl *remoteLister) process(item *workItem) (int, error) {
	absPath := item.path // TODO:: xload : check this for subdirectory mounting

	log.Debug("remoteLister::process : Reading remote dir %s", absPath)

	// this block will be executed only in the first list call for the remote directory
	// so haven't made the listBlocked variable atomic
	if !rl.listBlocked {
		log.Debug("remoteLister::process : Waiting for block-list-on-mount-sec before making the list call")
		err := waitForListTimeout()
		if err != nil {
			log.Err("remoteLister::process : unable to unmarshal block-list-on-mount-sec [%s]", err.Error())
			return 0, err
		}
		rl.listBlocked = true
	}

	marker := ""
	var cnt, iteration int
	for {
		// TODO:: xload : this fails when block list calls parameter in azstorage is non-zero
		entries, new_marker, err := rl.getRemote().StreamDir(internal.StreamDirOptions{
			Name:  absPath,
			Token: marker,
		})
		if err != nil {
			log.Err("remoteLister::process : Remote listing failed for %s [%s]", absPath, err.Error())
		}

		marker = new_marker
		cnt += len(entries)
		iteration++
		log.Debug("remoteLister::process : count: %d , iterations: %d", cnt, iteration)

		for _, entry := range entries {
			log.Debug("remoteLister::process : Iterating: %s, Is directory: %v", entry.Path, entry.IsDir())

			if entry.IsDir() {
				// create directory in local
				// spawn go routine for directory creation and then
				// adding to the input channel of the listing component
				// TODO:: xload : check how many threads can we spawn
				go func(name string) {
					localPath := filepath.Join(rl.path, name)
					err = rl.mkdir(localPath)
					// TODO:: xload : handle error
					if err != nil {
						log.Err("remoteLister::process : Failed to create directory [%s]", err.Error())
						return
					}

					// push the directory to input pool for its listing
					rl.getThreadPool().Schedule(&workItem{
						compName: rl.getName(),
						path:     name,
					})
				}(entry.Path)
			} else {
				// send file to the output channel for chunking
				rl.getNext().getThreadPool().Schedule(&workItem{
					compName: rl.getNext().getName(),
					path:     entry.Path,
					dataLen:  uint64(entry.Size),
				})
			}
		}

		if len(new_marker) == 0 {
			log.Debug("remoteLister::process : remote listing done for %s", absPath)
			break
		}
	}

	return cnt, nil
}

func (rl *remoteLister) mkdir(name string) error {
	log.Debug("remoteLister::mkdir : Creating local path: %s", name)
	return os.MkdirAll(name, 0777)
}
