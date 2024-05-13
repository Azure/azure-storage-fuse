package xload

import "github.com/Azure/azure-storage-fuse/v2/internal"

type xcomponent interface {
	init()
	start()
	stop()
	process(item *workItem) (int, error)
	getNext() xcomponent
	setNext(s xcomponent)
}

type xbase struct {
	pool   *ThreadPool
	remote internal.Component
	next   xcomponent
}

var _ xcomponent = &xbase{}

func (xb *xbase) init() {
}

func (xb *xbase) start() {
}

func (xb *xbase) stop() {
}

func (xb *xbase) process(item *workItem) (int, error) {
	return 0, nil
}

func (xb *xbase) getNext() xcomponent {
	return xb.next
}

func (xb *xbase) setNext(s xcomponent) {
	xb.next = s
}

func (xb *xbase) getThreadPool() *ThreadPool {
	return xb.pool
}

func (xb *xbase) getRemote() internal.Component {
	return xb.remote
}

// --------------------------------------------------------------------------------------------------------------------------------------------------

type xsplitter struct {
	xbase
}

var _ xcomponent = &xsplitter{}

// --------------------------------------------------------------------------------------------------------------------------------------------------

type xmanager struct {
	xbase
}

var _ xcomponent = &xmanager{}
