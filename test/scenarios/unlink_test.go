/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
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

package scenarios

// Test unlink on open, file deletion must be deferred until all file handles are closed.
// This is not supported yet commenting out.
// TODO: support this feature and enable the test.
// func TestUnlinkOnOpen(t *testing.T) {
// 	t.Parallel()
// 	filename := "testfile_unlink.txt"
// 	content := []byte("Hello, World!")
// 	content2 := []byte("Hello, Cosmos")
// 	for _, mnt := range mountpoints {
// 		filePath := filepath.Join(mnt, filename)
// 		//Open the file
// 		file, err := os.Create(filePath)
// 		assert.NoError(t, err)
// 		written, err := file.Write(content)
// 		assert.Equal(t, 13, written)
// 		assert.NoError(t, err)

// 		// Delete the file
// 		err = os.Remove(filePath)
// 		assert.NoError(t, err)

// 		// Read the content of the file after deleting the file.
// 		readContent := make([]byte, len(content))
// 		_, err = file.ReadAt(readContent, 0)
// 		assert.NoError(t, err)
// 		assert.Equal(t, string(content), string(readContent))

// 		err = file.Close()
// 		assert.NoError(t, err)

// 		// Open the file again
// 		_, err = os.Open(filePath)
// 		assert.Error(t, err)
// 		if err != nil {
// 			assert.Contains(t, err.Error(), "no such file or directory")
// 		}

// 		// Write to the file
// 		err = os.WriteFile(filePath, content2, 0644)
// 		assert.NoError(t, err)

// 		file2, err := os.Open(filePath)
// 		assert.NoError(t, err)

// 		// This read should be served from the newly created file
// 		_, err = file2.Read(readContent)
// 		assert.NoError(t, err)
// 		assert.Equal(t, string(content2), string(readContent))
// 	}
// 	checkFileIntegrity(t, filename)
// 	removeFiles(t, filename)
// }
