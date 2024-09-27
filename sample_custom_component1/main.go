package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/external"
)

// SAMPLE EXTERNAL COMPONENT IMPLEMENTATION
// This is a sample external component implementation that can be used as a reference to implement external components.
// The external component should implement the external.Component interface.
const (
	CompName = "sample_custom_component1"
	Mb       = 1024 * 1024
)

var _ external.Component = &sample_custom_component1{}

func (e *sample_custom_component1) SetName(name string) {
	e.BaseComponent.SetName(name)
}

func (e *sample_custom_component1) SetNextComponent(nc external.Component) {
	e.BaseComponent.SetNextComponent(nc)
}

func GetExternalComponent() (string, func() external.Component) {
	return CompName, NewexternalComponent
}

func NewexternalComponent() external.Component {
	comp := &sample_custom_component1{}
	comp.SetName(CompName)
	return comp
}

type sample_custom_component1 struct {
	blockSize int64
	external.BaseComponent
}

type sample_custom_component1Options struct {
	BlockSize int64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
}

func (e *sample_custom_component1) Configure(isParent bool) error {
	log.Trace("sample_custom_component1::Configure")
	conf := sample_custom_component1Options{}
	err := config.UnmarshalKey(e.Name(), &conf)
	if err != nil {
		log.Err("sample_custom_component1::Configure : config error [invalid config attributes]")
		return fmt.Errorf("error reading config for %s: %w", e.Name(), err)
	}
	if config.IsSet(e.Name()+".block-size-mb") && conf.BlockSize > 0 {
		e.blockSize = conf.BlockSize * int64(Mb)
	}
	return nil
}

func (e *sample_custom_component1) CreateFile(opt external.CreateFileOptions) (*external.Handle, error) {
	log.Trace("sample_custom_component1::CreateFile : %s", opt.Name)
	handle, err := e.NextComponent().CreateFile(opt)
	if err != nil {
		log.Err("sample_custom_component1::CreateFile failed: %v", err)
		return nil, err
	}
	return handle, nil
}

func (e *sample_custom_component1) CreateDir(opt external.CreateDirOptions) error {
	log.Trace("sample_custom_component1::CreateDir : %s", opt.Name)
	err := e.NextComponent().CreateDir(opt)
	if err != nil {
		log.Err("sample_custom_component1::CreateDir failed: %v", err)
		return err
	}
	return nil
}

func (e *sample_custom_component1) StreamDir(options external.StreamDirOptions) ([]*external.ObjAttr, string, error) {
	log.Trace("sample_custom_component1::StreamDir : %s", options.Name)

	attr, token, err := e.NextComponent().StreamDir(options)
	if err != nil {
		log.Err("sample_custom_component1::StreamDir failed: %v", err)
		return nil, "", err
	}
	return attr, token, nil
}
func (e *sample_custom_component1) IsDirEmpty(options external.IsDirEmptyOptions) bool {
	log.Trace("test2::IsDirEmpty : %s", options.Name)
	empty := e.NextComponent().IsDirEmpty(options)
	return empty
}

func (e *sample_custom_component1) DeleteDir(opt external.DeleteDirOptions) error {
	log.Trace("sample_custom_component1::DeleteDir : %s", opt.Name)
	err := e.NextComponent().DeleteDir(opt)
	if err != nil {
		log.Err("sample_custom_component1::DeleteDir failed: %v", err)
		return err
	}
	return nil
}

func (e *sample_custom_component1) StageData(opt external.StageDataOptions) error {
	log.Trace("sample_custom_component1::StageData : %s", opt.Name)
	err := e.NextComponent().StageData(opt)
	if err != nil {
		log.Err("sample_custom_component1 StageData failed: %v", err)
		return err
	}
	return nil
}

func (e *sample_custom_component1) ReadInBuffer(opt external.ReadInBufferOptions) (length int, err error) {
	log.Trace("sample_custom_component1::ReadInBuffer : %s", opt.Handle.Path)
	n, err := e.NextComponent().ReadInBuffer(opt)
	if err != nil {
		log.Err("sample_custom_component1 ReadInBuffer failed: %v", err)
		return 0, err
	}
	return n, nil
}

func (e *sample_custom_component1) GetAttr(options external.GetAttrOptions) (attr *external.ObjAttr, err error) {
	log.Trace("sample_custom_component1::GetAttr : %s", options.Name)
	attr, err = e.NextComponent().GetAttr(options)
	if err != nil {
		if os.IsNotExist(err) {
			log.Trace("sample_custom_component1::GetAttr : File not found: %s", options.Name)
			return nil, syscall.ENOENT
		}
		log.Err("sample_custom_component1 GetAttr failed: %v", err)
		return nil, err
	}
	return attr, nil
}
