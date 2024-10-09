package xload

import (
	"os"
	"path/filepath"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// verify that the below types implement the xcomponent interfaces
var _ xcomponent = &xlister{}
var _ xcomponent = &xlocal{}
var _ xcomponent = &xremote{}

// verify that the below types implement the xenumerator interfaces
var _ xenumerator = &xlocal{}
var _ xenumerator = &xremote{}

type xlister struct {
	xbase
	path string // base path of the directory to be listed
}

type xenumerator interface {
	mkdir(name string) error
}

// --------------------------------------------------------------------------------------------------------

type xlocal struct {
	xlister
}

func newXLocalLister(path string, remote internal.Component) (*xlocal, error) {
	ll := &xlocal{
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

func (ll *xlocal) init() {
	ll.pool = newThreadPool(MAX_LISTER, ll.process)
	if ll.pool == nil {
		log.Err("xlister::init : fail to init thread pool")
	}
}

func (ll *xlocal) start() {
	ll.getThreadPool().Start()
	ll.getThreadPool().Schedule(&workItem{})
}

func (ll *xlocal) stop() {
	if ll.getThreadPool() != nil {
		ll.getThreadPool().Stop()
	}
	ll.getNext().stop()
}

func (ll *xlocal) process(item *workItem) (int, error) {
	absPath := filepath.Join(ll.path, item.path)

	log.Trace("xlister::process : Reading local dir %s", absPath)

	entries, err := os.ReadDir(absPath)
	if err != nil {
		log.Err("xlister::process : [%s]", err.Error())
		return 0, err
	}

	for _, entry := range entries {
		relPath := filepath.Join(item.path, entry.Name())
		log.Trace("xlister::process : Iterating: %s, Is directory: %v", relPath, entry.IsDir())

		if entry.IsDir() {
			// spawn go routine for directory creation and then
			// adding to the input channel of the listing component
			go func(name string) {
				err = ll.mkdir(name)
				// TODO:: xload : handle error
				if err != nil {
					log.Err("xlister::process : Failed to create directory [%s]", err.Error())
					return
				}

				ll.getThreadPool().Schedule(&workItem{
					path: name,
				})
			}(relPath)

		} else {
			info, err := os.Stat(filepath.Join(absPath, entry.Name()))
			if err == nil {
				// send file to the output channel for chunking
				ll.getNext().getThreadPool().Schedule(&workItem{
					path:    relPath,
					dataLen: uint64(info.Size()),
				})
			} else {
				log.Err("xlister::process : Failed to get stat of %v", relPath)
			}
		}
	}

	return len(entries), nil
}

func (ll *xlocal) mkdir(name string) error {
	// create directory in container
	return ll.getRemote().CreateDir(internal.CreateDirOptions{
		Name: name,
		Mode: 0777,
	})
}

// --------------------------------------------------------------------------------------------------------

type xremote struct {
	xlister
}

func newXRemoteLister(path string, remote internal.Component) (*xremote, error) {
	rl := &xremote{
		xlister: xlister{
			path: path,
			xbase: xbase{
				remote: remote,
			},
		},
	}
	rl.init()
	return rl, nil
}

func (rl *xremote) init() {
	rl.pool = newThreadPool(MAX_LISTER, rl.process)
	if rl.pool == nil {
		log.Err("xlister::init : fail to init thread pool")
	}
}

func (rl *xremote) start() {
	rl.getThreadPool().Start()
	rl.getThreadPool().Schedule(&workItem{})
}

func (rl *xremote) stop() {
	if rl.getThreadPool() != nil {
		rl.getThreadPool().Stop()
	}
	rl.getNext().stop()
}

func (rl *xremote) process(item *workItem) (int, error) {
	absPath := item.path // TODO:: xload : check this for subdirectory mounting

	log.Trace("xlister::process : Reading remote dir %s", absPath)

	marker := ""
	var cnt, iteration int
	for {
		// TODO:: xload : this fails when block list calls parameter in azstorage is non-zero
		entries, new_marker, err := rl.getRemote().StreamDir(internal.StreamDirOptions{
			Name:  absPath,
			Token: marker,
		})
		if err != nil {
			log.Err("xlister::process : Remote listing failed for %s [%s]", absPath, err.Error())
		}

		marker = new_marker
		cnt += len(entries)
		iteration++
		log.Debug("xlister::process : count: %d , iterations: %d", cnt, iteration)

		for _, entry := range entries {
			log.Trace("xlister::process : Iterating: %s, Is directory: %v", entry.Path, entry.IsDir())

			if entry.IsDir() {
				// create directory in local
				// spawn go routine for directory creation and then
				// adding to the input channel of the listing component
				// TODO: check how many threads can we spawn
				go func(name string) {
					localPath := filepath.Join(rl.path, name)
					err = rl.mkdir(localPath)
					// TODO:: xload : handle error
					if err != nil {
						log.Err("xlister::process : Failed to create directory [%s]", err.Error())
						return
					}

					// push the directory to input pool for its listing
					rl.getThreadPool().Schedule(&workItem{
						path: name,
					})
				}(entry.Path)
			} else {
				// send file to the output channel for chunking
				rl.getNext().getThreadPool().Schedule(&workItem{
					path:    entry.Path,
					dataLen: uint64(entry.Size),
				})
			}
		}

		if len(new_marker) == 0 {
			log.Debug("list::readDir : remote listing done for %s", absPath)
			break
		}
	}

	return cnt, nil
}

func (rl *xremote) mkdir(name string) error {
	log.Trace("xlister::mkdir : Creating local path: %s", name)
	return os.MkdirAll(name, 0777)
}
