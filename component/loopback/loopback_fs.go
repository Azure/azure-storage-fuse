/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package loopback

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

//LoopbackFS component Config specifications:
//
//	loopbackfs:
//		path: <valid-path>
//

const compName = "loopbackfs"

type LoopbackFS struct {
	internal.BaseComponent

	path string
}

var _ internal.Component = &LoopbackFS{}

type LoopbackFSOptions struct {
	Path string `config:"path"`
}

func (lfs *LoopbackFS) Configure(_ bool) error {
	conf := LoopbackFSOptions{}
	err := config.UnmarshalKey(compName, &conf)
	if err != nil {
		log.Err("LoopbackFS: config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [%s]", lfs.Name(), err)
	}
	if _, err := os.Stat(conf.Path); os.IsNotExist(err) {
		err = os.MkdirAll(conf.Path, os.FileMode(0777))
		if err != nil {
			log.Err("LoopbackFS: config error [%s]", err)
			return fmt.Errorf("config error in %s [%s]", lfs.Name(), err)
		}
		lfs.path = conf.Path
	} else {
		lfs.path = conf.Path
	}
	return nil
}

func (lfs *LoopbackFS) Name() string {
	return compName
}

func (lfs *LoopbackFS) Start(ctx context.Context) error {
	log.Info("Started Loopback FS")
	return nil
}

func (lfs *LoopbackFS) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.Consumer()
}

func (lfs *LoopbackFS) CreateDir(options internal.CreateDirOptions) error {
	log.Trace("LoopbackFS::CreateDir : name=%s", options.Name)
	dirPath := filepath.Join(lfs.path, options.Name)
	return os.Mkdir(dirPath, options.Mode)
}

func (lfs *LoopbackFS) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("LoopbackFS::DeleteDir : name=%s", options.Name)
	dirPath := filepath.Join(lfs.path, options.Name)
	return os.Remove(dirPath)
}

func (lfs *LoopbackFS) IsDirEmpty(options internal.IsDirEmptyOptions) bool {
	log.Trace("LoopbackFS::IsDirEmpty : name=%s", options.Name)
	path := filepath.Join(lfs.path, options.Name)
	f, err := os.Open(path)
	if err != nil {
		log.Err("LoopbackFS::IsDirEmpty : error opening path [%s]", err)
		return false
	}
	_, err = f.Readdirnames(1)
	return err == io.EOF
}

func (lfs *LoopbackFS) ReadDir(options internal.ReadDirOptions) ([]*internal.ObjAttr, error) {
	log.Trace("LoopbackFS::ReadDir : name=%s", options.Name)
	attrList := make([]*internal.ObjAttr, 0)
	path := filepath.Join(lfs.path, options.Name)

	log.Debug("LoopbackFS::ReadDir : requested for %s", path)
	files, err := os.ReadDir(path)
	if err != nil {
		log.Err("LoopbackFS::ReadDir : error[%s]", err)
		return nil, err
	}
	log.Debug("LoopbackFS::ReadDir : on %s returned %d items", path, len(files))

	for _, file := range files {
		info, _ := file.Info()

		attr := &internal.ObjAttr{
			Path:  filepath.Join(options.Name, file.Name()),
			Name:  file.Name(),
			Size:  info.Size(),
			Mode:  info.Mode(),
			Mtime: info.ModTime(),
		}
		attr.Flags.Set(internal.PropFlagModeDefault)

		if file.IsDir() {
			attr.Flags.Set(internal.PropFlagIsDir)
		}

		attrList = append(attrList, attr)
	}
	return attrList, nil
}

// TODO: we can make it more intricate by generating a token and splitting streamed dir mimicking storage
func (lfs *LoopbackFS) StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	if options.Token == "na" {
		return nil, "", nil
	}
	log.Trace("LoopbackFS::StreamDir : name=%s", options.Name)
	attrList := make([]*internal.ObjAttr, 0)
	path := filepath.Join(lfs.path, options.Name)

	log.Debug("LoopbackFS::StreamDir : requested for %s", path)
	files, err := os.ReadDir(path)
	if err != nil {
		log.Err("LoopbackFS::StreamDir : error[%s]", err)
		return nil, "", err
	}
	log.Debug("LoopbackFS::StreamDir : on %s returned %d items", path, len(files))

	for _, file := range files {
		info, _ := file.Info()
		attr := &internal.ObjAttr{
			Path:  filepath.Join(options.Name, file.Name()),
			Name:  file.Name(),
			Size:  info.Size(),
			Mode:  info.Mode(),
			Mtime: info.ModTime(),
		}
		attr.Flags.Set(internal.PropFlagModeDefault)

		if file.IsDir() {
			attr.Flags.Set(internal.PropFlagIsDir)
		}

		attrList = append(attrList, attr)
	}
	return attrList, "", nil
}

func (lfs *LoopbackFS) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("LoopbackFS::RenameDir : %s -> %s", options.Src, options.Dst)
	oldPath := filepath.Join(lfs.path, options.Src)
	newPath := filepath.Join(lfs.path, options.Dst)

	return os.Rename(oldPath, newPath)
}

func (lfs *LoopbackFS) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("LoopbackFS::CreateFile : name=%s", options.Name)

	if options.Name == "FailThis" {
		return nil, fmt.Errorf("LoopbackFS::CreateFile : Failed to create file %s", options.Name)
	}

	path := filepath.Join(lfs.path, options.Name)

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, options.Mode)
	if err != nil {
		log.Err("LoopbackFS::CreateFile : error %s", err)
		return nil, err
	}
	handle := handlemap.NewHandle(options.Name)
	handle.SetFileObject(f)

	return handle, nil
}

func (lfs *LoopbackFS) CreateLink(options internal.CreateLinkOptions) error {
	log.Trace("LoopbackFS::CreateLink : name=%s", options.Name)
	path := filepath.Join(lfs.path, options.Name)

	err := os.Symlink(options.Target, path)

	return err
}

func (lfs *LoopbackFS) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("LoopbackFS::DeleteFile : name=%s", options.Name)
	path := filepath.Join(lfs.path, options.Name)
	return os.Remove(path)
}

func (lfs *LoopbackFS) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("LoopbackFS::OpenFile : name=%s", options.Name)
	path := filepath.Join(lfs.path, options.Name)
	log.Debug("LoopbackFS::OpenFile : requested for %s", options.Name)
	f, err := os.OpenFile(path, options.Flags, options.Mode)
	if err != nil {
		log.Err("LoopbackFS::OpenFile : error [%s]", err)
		return nil, err
	}
	handle := handlemap.NewHandle(options.Name)
	handle.SetFileObject(f)
	return handle, nil
}

func (lfs *LoopbackFS) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("LoopbackFS::CloseFile : name=%s", options.Handle.Path)

	f := options.Handle.GetFileObject()
	if f == nil {
		log.Err("LoopbackFS::CloseFile : error [file not available]")
		return syscall.EBADF
	}

	return f.Close()
}

func (lfs *LoopbackFS) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("LoopbackFS::RenameFile : %s -> %s", options.Src, options.Dst)
	oldPath := filepath.Join(lfs.path, options.Src)
	newPath := filepath.Join(lfs.path, options.Dst)
	return os.Rename(oldPath, newPath)
}

func (lfs *LoopbackFS) ReadFile(options internal.ReadFileOptions) ([]byte, error) {
	log.Trace("LoopbackFS::ReadFile : name=%s", options.Handle.Path)
	f := options.Handle.GetFileObject()

	options.Handle.RLock()
	defer options.Handle.RUnlock()

	if f == nil {
		log.Err("LoopbackFS::ReadFile : error [invalid file object]")
		return nil, os.ErrInvalid
	}

	info, err := f.Stat()
	if err != nil {
		log.Err("LoopbackFS::ReadFile : error [%s]", err)
		return nil, err
	}
	data := make([]byte, info.Size())
	n, err := f.Read(data)
	if int64(n) != info.Size() {
		log.Err("LoopbackFS::ReadFile : error [could not read entire file]")
		return nil, err
	}
	return data, nil
}

func (lfs *LoopbackFS) ReadLink(options internal.ReadLinkOptions) (string, error) {
	log.Trace("LoopbackFS::ReadLink : name=%s", options.Name)
	path := filepath.Join(lfs.path, options.Name)
	targetPath, err := os.Readlink(path)
	if err != nil {
		log.Err("LoopbackFS::ReadLink : error [%s]", err)
		return "", err
	}
	return strings.TrimPrefix(targetPath, lfs.path), nil
}

func (lfs *LoopbackFS) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	log.Trace("LoopbackFS::ReadInBuffer : name=%s", options.Handle.Path)
	f := options.Handle.GetFileObject()

	if f == nil {
		f1, err := os.OpenFile(filepath.Join(lfs.path, options.Handle.Path), os.O_RDONLY, 0777)
		if err != nil {
			return 0, nil
		}

		n, err := f1.ReadAt(options.Data, options.Offset)
		f1.Close()
		return n, err
	}

	options.Handle.RLock()
	defer options.Handle.RUnlock()

	if f == nil {
		log.Err("LoopbackFS::ReadInBuffer : error [invalid file object]")
		return 0, os.ErrInvalid
	}
	return f.ReadAt(options.Data, options.Offset)
}

func (lfs *LoopbackFS) WriteFile(options internal.WriteFileOptions) (int, error) {
	log.Trace("LoopbackFS::WriteFile : name=%s", options.Handle.Path)
	f := options.Handle.GetFileObject()

	options.Handle.Lock()
	defer options.Handle.Unlock()

	if f == nil {
		log.Err("LoopbackFS::WriteFile : error [invalid file object]")
		return 0, os.ErrInvalid
	}
	options.Handle.Flags.Set(handlemap.HandleFlagDirty)
	return f.WriteAt(options.Data, options.Offset)
}

func (lfs *LoopbackFS) TruncateFile(options internal.TruncateFileOptions) error {
	log.Trace("LoopbackFS::TruncateFile : name=%s", options.Name)
	fsPath := filepath.Join(lfs.path, options.Name)
	return os.Truncate(fsPath, options.Size)
}

func (lfs *LoopbackFS) FlushFile(options internal.FlushFileOptions) error {
	log.Trace("LoopbackFS::FlushFile : name=%s", options.Handle.Path)
	f := options.Handle.GetFileObject()
	if f == nil {
		log.Err("LoopbackFS::FlushFile : error [file not open]")
		return os.ErrClosed
	}

	return nil
}

func (lfs *LoopbackFS) ReleaseFile(options internal.ReleaseFileOptions) error {
	log.Trace("LoopbackFS::ReleaseFile : name=%s", options.Handle.Path)
	f := options.Handle.GetFileObject()
	if f == nil {
		log.Err("LoopbackFS::ReleaseFile : error [file not open]")
		return fmt.Errorf("LoopbackFS::ReleaseFile : %s file not open", options.Handle.Path)
	}
	return nil
}

func (lfs *LoopbackFS) UnlinkFile(options internal.UnlinkFileOptions) error {
	log.Trace("LoopbackFS::UnlinkFile : name=%s", options.Name)
	path := filepath.Join(lfs.path, options.Name)
	_, err := os.Lstat(path)
	if err != nil {
		log.Err("LoopbackFS::UnlinkFile : error [%s]", err)
		return err
	}
	return err
}

func (lfs *LoopbackFS) CopyToFile(options internal.CopyToFileOptions) error {
	log.Trace("LoopbackFS::CopyToFile : name=%s", options.Name)
	path := filepath.Join(lfs.path, options.Name)
	fsrc, err := os.Open(path)
	if err != nil {
		log.Err("LoopbackFS::CopyToFile : error opening [%s]", err)
		return err
	}
	_, err = io.Copy(options.File, fsrc)
	if err != nil {
		log.Err("LoopbackFS::CopyToFile : error copying [%s]", err)
		return err
	}
	return nil
}

func (lfs *LoopbackFS) CopyFromFile(options internal.CopyFromFileOptions) error {
	log.Trace("LoopbackFS::CopyFromFile : name=%s", options.Name)
	path := filepath.Join(lfs.path, options.Name)
	fdst, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666))
	if err != nil {
		log.Err("LoopbackFS::CopyFromFile : error opening [%s]", err)
		return err
	}
	_, err = io.Copy(fdst, options.File)
	if err != nil {
		log.Err("LoopbackFS::CopyFromFile : error copying [%s]", err)
		return err
	}
	return nil
}

func (lfs *LoopbackFS) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	log.Trace("LoopbackFS::GetAttr : name=%s", options.Name)
	path := filepath.Join(lfs.path, options.Name)
	info, err := os.Lstat(path)
	if err != nil {
		log.Err("LoopbackFS::GetAttr : error [%s]", err)
		return &internal.ObjAttr{}, err
	}
	attr := &internal.ObjAttr{
		Path:  options.Name,
		Name:  info.Name(),
		Size:  info.Size(),
		Mode:  info.Mode(),
		Mtime: info.ModTime(),
	}
	attr.Flags.Set(internal.PropFlagModeDefault)

	if info.Mode()&os.ModeSymlink != 0 {
		_, err := os.Readlink(path)
		if err != nil {
			log.Err("LoopbackFS::GetAttr : could not find target of symlink %s", options.Name)
			return attr, err
		}
		attr.Flags.Set(internal.PropFlagSymlink)
	} else if info.IsDir() {
		attr.Flags.Set(internal.PropFlagIsDir)
	}
	return attr, nil
}

func (lfs *LoopbackFS) Chmod(options internal.ChmodOptions) error {
	log.Trace("LoopbackFS::Chmod : name=%s", options.Name)
	path := filepath.Join(lfs.path, options.Name)
	return os.Chmod(path, options.Mode)
}

func (lfs *LoopbackFS) Chown(options internal.ChownOptions) error {
	log.Trace("LoopbackFS::Chown : name=%s", options.Name)
	path := filepath.Join(lfs.path, options.Name)
	return os.Chown(path, options.Owner, options.Group)
}

func (lfs *LoopbackFS) StageData(options internal.StageDataOptions) error {
	log.Trace("LoopbackFS::StageData : name=%s, id=%s", options.Name, options.Id)
	path := fmt.Sprintf("%s_%s", filepath.Join(lfs.path, options.Name), strings.ReplaceAll(options.Id, "/", "_"))
	return os.WriteFile(path, options.Data, 0777)
}

func (lfs *LoopbackFS) CommitData(options internal.CommitDataOptions) error {
	log.Trace("LoopbackFS::StageData : name=%s", options.Name)

	mainFilepath := filepath.Join(lfs.path, options.Name)

	blob, err := os.OpenFile(mainFilepath, os.O_RDWR|os.O_CREATE, os.FileMode(0777))
	if err != nil {
		log.Err("LoopbackFS::CommitData : error opening [%s]", err)
		return err
	}

	if len(options.List) == 0 {
		err = blob.Truncate(0)
		if err != nil {
			return err
		}
	}

	for idx, id := range options.List {
		path := fmt.Sprintf("%s_%s", filepath.Join(lfs.path, options.Name), strings.ReplaceAll(id, "/", "_"))
		info, err := os.Lstat(path)
		if err == nil {
			block, err := os.OpenFile(path, os.O_RDONLY, os.FileMode(0666))
			if err != nil {
				return err
			}

			data := make([]byte, info.Size())
			n, err := block.Read(data)
			if int64(n) != info.Size() {
				log.Err("LoopbackFS::CommitData : error [could not read entire file]")
				return err
			}

			n, err = blob.WriteAt(data, int64(idx*(int)(options.BlockSize)))
			if err != nil {
				return err
			}
			if int64(n) != info.Size() {
				log.Err("LoopbackFS::CommitData : error [could not write file]")
				return err
			}

			err = block.Close()
			if err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}

	// delete the staged files
	for _, id := range options.List {
		path := fmt.Sprintf("%s_%s", filepath.Join(lfs.path, options.Name), strings.ReplaceAll(id, "/", "_"))
		_ = os.Remove(path)
	}

	err = blob.Close()
	return err
}

func (lfs *LoopbackFS) GetCommittedBlockList(name string) (*internal.CommittedBlockList, error) {
	mainFilepath := filepath.Join(lfs.path, name)

	info, err := os.Lstat(mainFilepath)
	if err != nil {
		return nil, err
	}

	blockSize := uint64(1 * 1024 * 1024)
	blocks := info.Size() / (int64)(blockSize)
	list := make(internal.CommittedBlockList, 0)

	for i := int64(0); i < blocks; i++ {
		list = append(list, internal.CommittedBlock{
			Id:     fmt.Sprintf("%d", i),
			Offset: i * (int64)(blockSize),
			Size:   blockSize,
		})
	}

	return &list, nil
}

func (lfs *LoopbackFS) WriteFromBuffer(options internal.WriteFromBufferOptions) error {
	log.Trace("LoopbackFS::WriteFromBuffer : name=%s", options.Name)
	path := filepath.Join(lfs.path, options.Name)
	return os.WriteFile(path, options.Data, os.FileMode(0666))
}

func NewLoopbackFSComponent() internal.Component {
	lfs := &LoopbackFS{}
	lfs.SetName(compName)
	return lfs
}

func init() {
	internal.AddComponent(compName, NewLoopbackFSComponent)
}
