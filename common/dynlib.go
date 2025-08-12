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

import "unsafe"

// DynamicLibrary represents a handle to a dynamically loaded library
type DynamicLibrary struct {
	handle uintptr
}

// LoadLibrary loads a dynamic library from the given path
func LoadLibrary(path string) (*DynamicLibrary, error) {
	handle, err := loadLibrary(path)
	if err != nil {
		return nil, err
	}
	return &DynamicLibrary{handle: handle}, nil
}

// GetSymbol retrieves a symbol (function pointer) from the library
func (dl *DynamicLibrary) GetSymbol(name string) (unsafe.Pointer, error) {
	return getSymbol(dl.handle, name)
}

// Close closes the dynamic library
func (dl *DynamicLibrary) Close() error {
	if dl.handle == 0 {
		return nil
	}
	err := closeLibrary(dl.handle)
	dl.handle = 0
	return err
}