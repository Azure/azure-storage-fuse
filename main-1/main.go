package main

import (
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

const (
	CompName = "test1"
)

func (e *test1) SetName(name string) {
	e.BaseComponent.SetName(name)
}

func (e *test1) SetNextComponent(nc internal.Component) {
	e.BaseComponent.SetNextComponent(nc)
}
func InitPlugin() {
	internal.AddComponent("test1", NewCustomComponent)
}
func NewCustomComponent() internal.Component {
	comp := &test1{}
	comp.SetName(CompName)
	return comp
}

type test1 struct {
	internal.BaseComponent
}

func (e *test1) StageData(opt internal.StageDataOptions) error {
	log.Info("in test1 StageData")
	return nil
}
func (e *test1) Configure(isParent bool) error {
	log.Info("in test1 Configure")
	return nil
}

func (e *test1) GetAttr(options internal.GetAttrOptions) (attr *internal.ObjAttr, err error) {
	log.Info("in test1 GetAttr")
	return nil, nil
}
