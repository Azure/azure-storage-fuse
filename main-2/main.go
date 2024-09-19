package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/external"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// SAMPLE external COMPONENT IMPLEMENTATION
// This is a sample external component implementation that can be used as a reference to implement external components.
// The external component should implement the external.Component interface.
const (
	CompName = "test2"
	Mb       = 1024 * 1024
)

var _ external.Component = &test2{}

func (e *test2) SetName(name string) {
	e.BaseComponent.SetName(name)
}

func (e *test2) SetNextComponent(nc external.Component) {
	e.BaseComponent.SetNextComponent(nc)
}
func InitPlugin() {
	external.AddComponent(CompName, NewexternalComponent)
}
func NewexternalComponent() external.Component {
	comp := &test2{}
	comp.SetName(CompName)
	return comp
}

type test2 struct {
	blockSize    int64
	externalPath string
	external.BaseComponent
}

type test2Options struct {
	BlockSize int64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
}

func (e *test2) Configure(isParent bool) error {
	log.Info("test2:: Configure")
	externalPath := os.Getenv("EXTERNAL_PATH")
	if externalPath == "" {
		log.Err("EXTERNAL_PATH not set")
		return fmt.Errorf("EXTERNAL_PATH not set")
	}
	e.externalPath = externalPath
	conf := test2Options{}
	err := config.UnmarshalKey(e.Name(), &conf)
	if err != nil {
		log.Err("test2::Configure : config error [invalid config attributes]")
		return fmt.Errorf("error reading config for %s: %w", e.Name(), err)
	}
	if config.IsSet(e.Name()+".block-size-mb") && conf.BlockSize > 0 {
		e.blockSize = conf.BlockSize * int64(Mb)
	}
	return nil
}

func (e *test2) CreateFile(options external.CreateFileOptions) (*external.Handle, error) {
	log.Info("test2::CreateFile")
	filePath := filepath.Join(e.externalPath, options.Name)
	fileHandle, err := os.OpenFile(filePath, os.O_CREATE, 0777)
	if err != nil {
		log.Err("Failed to create file %s", filePath)
		return nil, err
	}
	defer fileHandle.Close()
	handle := &external.Handle{
		Path: options.Name,
	}
	return handle, nil
}

func (e *test2) CreateDir(options external.CreateDirOptions) error {
	log.Info("test2::CreateDir")
	dirPath := filepath.Join(e.externalPath, options.Name)
	err := os.MkdirAll(dirPath, 0777)
	if err != nil {
		log.Err("Failed to create directory %s", dirPath)
		return err
	}
	return nil
}

func (e *test2) StreamDir(options external.StreamDirOptions) ([]*external.ObjAttr, string, error) {
	log.Info("test2::StreamDir")
	var objAttrs []*internal.ObjAttr
	path := formatListDirName(options.Name)
	files, err := os.ReadDir(filepath.Join(e.externalPath, path))
	if err != nil {
		log.Trace("test1::StreamDir : Error reading directory %s : %s", path, err.Error())
		return nil, "", err
	}

	for _, file := range files {
		attr, err := e.GetAttr(internal.GetAttrOptions{Name: path + file.Name()})
		if err != nil {
			if err != syscall.ENOENT {
				log.Trace("test1::StreamDir : Error getting file attributes: %s", err.Error())
				return objAttrs, "", err
			}
			log.Trace("test1::StreamDir : File not found: %s", file.Name())
			continue
		}

		objAttrs = append(objAttrs, attr)
	}

	return objAttrs, "", nil
}
func (e *test2) IsDirEmpty(options external.IsDirEmptyOptions) bool {
	log.Info("test2::IsDirEmpty")
	files, err := os.ReadDir(filepath.Join(e.externalPath, options.Name))
	if err != nil {
		log.Err("Failed to read directory %s", options.Name)
		return false
	}
	return len(files) == 0
}

func (e *test2) DeleteDir(options external.DeleteDirOptions) error {
	log.Info("test2::DeleteDir")
	dirPath := filepath.Join(e.externalPath, options.Name)
	err := os.RemoveAll(dirPath)
	if err != nil {
		log.Err("Failed to delete directory %s", dirPath)
		return err
	}
	return nil
}

func (e *test2) StageData(opt external.StageDataOptions) error {
	log.Info("test2:: StageData")
	filePath := filepath.Join(e.externalPath, opt.Name)
	fileHandle, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		log.Err("Failed to open file %s", filePath)
		return err
	}
	defer fileHandle.Close()

	fileOffset := int64(opt.Offset) * e.blockSize

	_, err = fileHandle.WriteAt(opt.Data, fileOffset)

	if err != nil {
		log.Err("Failed to write to file %s at offset %d", filePath, fileOffset)
		return err
	}

	return nil
}

func (e *test2) ReadInBuffer(opt external.ReadInBufferOptions) (length int, err error) {
	log.Info("test2:: ReadInBuffer")

	filePath := filepath.Join(e.externalPath, opt.Handle.Path)
	fileHandle, err := os.OpenFile(filePath, os.O_RDONLY, 0777)
	if err != nil {
		log.Err("Failed to open file %s", filePath)
		return 0, err
	}
	defer fileHandle.Close()

	n, err := fileHandle.ReadAt(opt.Data, opt.Offset)
	if err != nil {
		log.Err("Failed to read from file %s at offset %d", filePath, opt.Offset)
		return 0, err
	}
	return n, nil
}

func (e *test2) GetAttr(options external.GetAttrOptions) (attr *external.ObjAttr, err error) {
	log.Info("test2::GetAttr for %s", options.Name)
	fileAttr, err := os.Stat(filepath.Join(e.externalPath, options.Name))
	if err != nil {
		log.Trace("test2::GetAttr : Error getting file attributes: %s", err.Error())
		return &external.ObjAttr{}, err
	}

	// Populate the ObjAttr struct with the file info.
	attr = &external.ObjAttr{
		Mtime:  fileAttr.ModTime(), // Modified time
		Atime:  time.Now(),         // Access time (current time as approximation)
		Ctime:  fileAttr.ModTime(), // Change time (same as modified time in this case)
		Crtime: fileAttr.ModTime(), // Creation time (not available in Go, using modified time)
		Mode:   fileAttr.Mode(),    // Permissions
		Path:   options.Name,       // File path
		Name:   fileAttr.Name(),    // Base name of the path
		Size:   fileAttr.Size(),    // File size
	}
	if fileAttr.IsDir() {
		attr.Flags.Set(external.PropFlagIsDir)
	}
	return attr, nil
}

func formatListDirName(path string) string {
	// If we check the root directory, make sure we pass "" instead of "/"
	// If we aren't checking the root directory, then we want to extend the directory name so List returns all children and does not include the path itself.
	if path == "/" {
		path = ""
	} else if path != "" {
		path = external.ExtendDirName(path)
	}
	return path
}
