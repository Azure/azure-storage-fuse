//go:build windows

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

package common

import (
	"syscall"
	"unsafe"
)

var (
	kernel32dll        = syscall.NewLazyDLL("kernel32.dll")
	procLoadLibrary    = kernel32dll.NewProc("LoadLibraryW")
	procGetProcAddress = kernel32dll.NewProc("GetProcAddress")
	procFreeLibrary    = kernel32dll.NewProc("FreeLibrary")
)

func loadLibrary(path string) (uintptr, error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	
	ret, _, err := procLoadLibrary.Call(uintptr(unsafe.Pointer(pathPtr)))
	if ret == 0 {
		return 0, err
	}
	
	return ret, nil
}

func getSymbol(handle uintptr, name string) (unsafe.Pointer, error) {
	namePtr, err := syscall.BytePtrFromString(name)
	if err != nil {
		return nil, err
	}
	
	ret, _, err := procGetProcAddress.Call(handle, uintptr(unsafe.Pointer(namePtr)))
	if ret == 0 {
		return nil, err
	}
	
	return unsafe.Pointer(ret), nil
}

func closeLibrary(handle uintptr) error {
	ret, _, err := procFreeLibrary.Call(handle)
	if ret == 0 {
		return err
	}
	return nil
}