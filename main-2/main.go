package main

import (
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

const (
	CompName = "test2"
)

func (e *test2) SetName(name string) {
	e.BaseComponent.SetName(name)
}

func (e *test2) SetNextComponent(nc internal.Component) {
	e.BaseComponent.SetNextComponent(nc)
}
func InitPlugin() {
	internal.AddComponent("test2", NewCustomComponent)
}
func NewCustomComponent() internal.Component {
	comp := &test2{}
	comp.SetName(CompName)
	return comp
}

type test2 struct {
	internal.BaseComponent
}

func (e *test2) StageData(opt internal.StageDataOptions) error {
	log.Info("in test2 StageData")
	return nil
}
func (e *test2) Configure(isParent bool) error {
	log.Info("in test2 Configure")
	return nil
}

func (e *test2) GetAttr(options internal.GetAttrOptions) (attr *internal.ObjAttr, err error) {
	log.Info("in test2 GetAttr")
	return nil, nil
}
