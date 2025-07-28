package scenarios

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Specify Mountpoints to check the file integrity across filesystems.
// Specifying one Mountpoint will check all the files for the errors.
var mountpoints []string

func calculateMD5(t *testing.T, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() {
		err := file.Close()
		assert.Nil(t, err)
	}()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func checkFileIntegrity(t *testing.T, filename string) {
	if len(mountpoints) > 1 {
		var referenceMD5 string
		var referenceSize int64
		for i, mnt := range mountpoints {
			filePath := filepath.Join(mnt, filename)
			fi, err := os.Stat(filePath)
			assert.Nil(t, err)
			md5sum, err := calculateMD5(t, filePath)
			assert.Nil(t, err)

			if i == 0 {
				referenceMD5 = md5sum
				referenceSize = fi.Size()
			} else {
				assert.Equal(t, referenceMD5, md5sum, "File content mismatch between mountpoints")
				assert.Equal(t, referenceSize, fi.Size(), "File Size mismatch between mountpoints")
			}
		}
	}
}

func removeFiles(t *testing.T, filename string) {
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		err := os.Remove(filePath)
		assert.Nil(t, err)
	}
}

func TestFileOpen(t *testing.T) {
	t.Parallel()
	filename := "testfile_open.txt"
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.Nil(t, err)
		err = file.Close()
		assert.Nil(t, err)

		file, err = os.Open(filePath)
		assert.Nil(t, err)
		err = file.Close()
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

func TestFileRead(t *testing.T) {
	t.Parallel()
	filename := "testfile_read.txt"
	content := []byte("Hello, World!")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		err := os.WriteFile(filePath, content, 0644)
		assert.Nil(t, err)

		file, err := os.Open(filePath)
		assert.Nil(t, err)

		readContent := make([]byte, len(content))
		_, err = file.Read(readContent)
		assert.True(t, err == nil || err == io.EOF)

		assert.Equal(t, string(content), string(readContent))

		err = file.Close()
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

func TestFileWrite(t *testing.T) {
	t.Parallel()
	filename := "testfile_write.txt"
	content := []byte("Hello, World!")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.Nil(t, err)

		_, err = file.Write(content)
		assert.Nil(t, err)

		err = file.Close()
		assert.Nil(t, err)

		readContent, err := os.ReadFile(filePath)
		assert.Nil(t, err)

		assert.Equal(t, string(content), string(readContent))
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

func TestFsync(t *testing.T) {
	t.Parallel()
	filename := "testfile_fsync.txt"
	content := []byte("Hello, World!")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.Nil(t, err)

		_, err = file.Write(content)
		assert.Nil(t, err)

		err = file.Sync()
		assert.Nil(t, err)

		readContent, err := os.ReadFile(filePath)
		assert.Nil(t, err)

		assert.Equal(t, string(content), string(readContent))

		err = file.Close()
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

func TestFsyncWhileWriting(t *testing.T) {
	t.Parallel()
	var err error
	filename := "testfile_fsync_while_writing.txt"
	readBufSize := 4 * 1024
	content := make([]byte, readBufSize)
	_, err = io.ReadFull(rand.Reader, content)
	assert.Nil(t, err)
	expectedContent := make([]byte, 4*1024, 10*1024*1024)
	copy(expectedContent, content)
	actualContent := make([]byte, 10*1024*1024)
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.Nil(t, err)

		// Write 9MB data, for each 4K buffer do an fsync for each 4K buffer. do read the data after fsync with other handle.
		for i := 0; i*readBufSize < 9*1024*1024; i += 4 * 1024 {
			bytesWritten, err := file.Write(content)
			assert.Nil(t, err)
			assert.Equal(t, len(content), bytesWritten)

			// We cannot do fsync for every 4K write, as the test takes long time to finish
			// do it for every 512K
			if i%(512*1024) == 0 {
				err = file.Sync()
				assert.Nil(t, err)
			}

			file1, err := os.Open(filePath)
			assert.Nil(t, err)
			bytesRead, err := file1.Read(actualContent)
			assert.Equal(t, (i+1)*readBufSize, bytesRead)
			assert.Nil(t, err)
			err = file1.Close()
			assert.Nil(t, err)

			assert.Equal(t, expectedContent[:(i+1)*readBufSize], actualContent[:(i+1)*readBufSize])
			expectedContent = append(expectedContent, content...)
		}

		err = file.Close()
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Add Tests for reading and writing to the newly created blocks and modified blocks while truncate.
const (
	truncate int = iota
	ftruncate
)

// tests for truncate function which works on path
func FileTruncate(t *testing.T, filename string, initialSize int, finalSize int, call int) {
	content := make([]byte, initialSize)
	_, err := io.ReadFull(rand.Reader, content)
	assert.Nil(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		err := os.WriteFile(filePath, content, 0644)
		assert.Nil(t, err)

		if call == truncate {
			err = os.Truncate(filePath, int64(finalSize))
			assert.Nil(t, err)
		} else if call == ftruncate {
			file, _ := os.OpenFile(filePath, os.O_RDWR, 0644)
			assert.Nil(t, err)
			err = file.Truncate(int64(finalSize))
			assert.Nil(t, err)
			err = file.Close()
			assert.Nil(t, err)
		}

		readContent, err := os.ReadFile(filePath)
		assert.Nil(t, err)

		expectedContent := make([]byte, initialSize)
		copy(expectedContent, content)
		if finalSize > initialSize {
			expectedContent = append(expectedContent, make([]byte, finalSize-initialSize)...)
		} else {
			expectedContent = expectedContent[:finalSize]
		}
		assert.Equal(t, string(expectedContent), string(readContent))
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

func TestFileTruncateSameSize(t *testing.T) {
	t.Parallel()
	filename := "testfile_truncate_same_size.txt"
	FileTruncate(t, filename, 10, 10, truncate)
	FileTruncate(t, filename, 9*1024*1024, 9*1024*1024, truncate)
	FileTruncate(t, filename, 8*1024*1024, 8*1024*1024, truncate)
}

func TestFileTruncateShrink(t *testing.T) {
	t.Parallel()

	filename := "testfile_truncate_shrink.txt"
	var wg sync.WaitGroup

	// Define table tests
	tests := []struct {
		name       string
		initial    int
		final      int
		truncation int
	}{
		{fmt.Sprintf("%s_20_5_truncate", filename), 20, 5, truncate},
		{fmt.Sprintf("%s_10M_5K_truncate", filename), 10 * 1024 * 1024, 5 * 1024, truncate},
		{fmt.Sprintf("%s_20M_5K_truncate", filename), 20 * 1024 * 1024, 5 * 1024, truncate},
		{fmt.Sprintf("%s_30M_20M_truncate", filename), 30 * 1024 * 1024, 20 * 1024 * 1024, truncate},
		{fmt.Sprintf("%s_20_5_ftruncate", filename), 20, 5, ftruncate},
		{fmt.Sprintf("%s_10M_5K_ftruncate", filename), 10 * 1024 * 1024, 5 * 1024, ftruncate},
		{fmt.Sprintf("%s_20M_5K_ftruncate", filename), 20 * 1024 * 1024, 5 * 1024, ftruncate},
		{fmt.Sprintf("%s_30M_20M_ftruncate", filename), 30 * 1024 * 1024, 20 * 1024 * 1024, ftruncate},
	}

	// Add the number of test cases to the WaitGroup
	wg.Add(len(tests))

	// Iterate over the test cases
	for _, tt := range tests {
		go func(tt struct {
			name       string
			initial    int
			final      int
			truncation int
		}) {
			defer wg.Done()
			FileTruncate(t, tt.name, tt.initial, tt.final, tt.truncation)
		}(tt)
	}

	// Wait for all goroutines to complete
	wg.Wait()
}

func TestFileTruncateExpand(t *testing.T) {
	t.Parallel()

	filename := "testfile_truncate_expand.txt"
	var wg sync.WaitGroup

	// Define table tests
	tests := []struct {
		name       string
		initial    int
		final      int
		truncation int
	}{
		{fmt.Sprintf("%s_5_20_truncate", filename), 5, 20, truncate},
		{fmt.Sprintf("%s_5K_10M_truncate", filename), 5 * 1024, 10 * 1024 * 1024, truncate},
		{fmt.Sprintf("%s_5K_20M_truncate", filename), 5 * 1024, 20 * 1024 * 1024, truncate},
		{fmt.Sprintf("%s_20M_30M_truncate", filename), 20 * 1024 * 1024, 30 * 1024 * 1024, truncate},
		{fmt.Sprintf("%s_5_20_ftruncate", filename), 5, 20, ftruncate},
		{fmt.Sprintf("%s_5K_10M_ftruncate", filename), 5 * 1024, 10 * 1024 * 1024, ftruncate},
		{fmt.Sprintf("%s_5K_20M_ftruncate", filename), 5 * 1024, 20 * 1024 * 1024, ftruncate},
		{fmt.Sprintf("%s_20M_30M_ftruncate", filename), 20 * 1024 * 1024, 30 * 1024 * 1024, ftruncate},
	}

	// Add the number of test cases to the WaitGroup
	wg.Add(len(tests))

	// Iterate over the test cases
	for _, tt := range tests {
		go func(tt struct {
			name       string
			initial    int
			final      int
			truncation int
		}) {
			defer wg.Done()
			FileTruncate(t, tt.name, tt.initial, tt.final, tt.truncation)
		}(tt)
	}

	// Wait for all goroutines to complete
	wg.Wait()
}

func TestTruncateNoFile(t *testing.T) {
	t.Parallel()
	filename := "testfile_truncate_no_file.txt"

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		err := os.Truncate(filePath, 5)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "no such file or directory")
	}
}

func WriteTruncateClose(t *testing.T, filename string, writeSize int, truncSize int, call int) {
	content := make([]byte, writeSize)
	_, err := io.ReadFull(rand.Reader, content)
	assert.Nil(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.Nil(t, err)
		written, err := file.Write(content)
		assert.Nil(t, err)
		assert.Equal(t, writeSize, written)
		if call == truncate {
			err := os.Truncate(filePath, int64(truncSize))
			assert.Nil(t, err)
		} else {
			err := file.Truncate(int64(truncSize))
			assert.Nil(t, err)
		}
		err = file.Close()
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

func TestWriteTruncateClose(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup

	// Define table tests
	tests := []struct {
		name       string
		initial    int
		final      int
		truncation int
	}{
		{"testWriteTruncateClose1M7M_truncate", 1 * 1024 * 1024, 7 * 1024 * 1024, truncate},
		{"testWriteTruncateClose1M13M_truncate", 1 * 1024 * 1024, 13 * 1024 * 1024, truncate},
		{"testWriteTruncateClose1M20M_truncate", 1 * 1024 * 1024, 20 * 1024 * 1024, truncate},
		{"testWriteTruncateClose7M1M_truncate", 7 * 1024 * 1024, 1 * 1024 * 1024, truncate},
		{"testWriteTruncateClose13M1M_truncate", 13 * 1024 * 1024, 1 * 1024 * 1024, truncate},
		{"testWriteTruncateClose20M1M_truncate", 20 * 1024 * 1024, 1 * 1024 * 1024, truncate},
		{"testWriteTruncateClose1M7M_ftruncate", 1 * 1024 * 1024, 7 * 1024 * 1024, ftruncate},
		{"testWriteTruncateClose1M13M_ftruncate", 1 * 1024 * 1024, 13 * 1024 * 1024, ftruncate},
		{"testWriteTruncateClose1M20M_ftruncate", 1 * 1024 * 1024, 20 * 1024 * 1024, ftruncate},
		{"testWriteTruncateClose7M1M_ftruncate", 7 * 1024 * 1024, 1 * 1024 * 1024, ftruncate},
		{"testWriteTruncateClose13M1M_ftruncate", 13 * 1024 * 1024, 1 * 1024 * 1024, ftruncate},
		{"testWriteTruncateClose20M1M_ftruncate", 20 * 1024 * 1024, 1 * 1024 * 1024, ftruncate},
	}

	// Add the number of test cases to the WaitGroup
	wg.Add(len(tests))

	// Iterate over the test cases
	for _, tt := range tests {
		go func(tt struct {
			name       string
			initial    int
			final      int
			truncation int
		}) {
			defer wg.Done()
			WriteTruncateClose(t, tt.name, tt.initial, tt.final, tt.truncation)
		}(tt)
	}

	// Wait for all goroutines to complete
	wg.Wait()
}

func TestWrite10MB(t *testing.T) {
	t.Parallel()
	filename := "testfile_write_10mb.txt"
	content := make([]byte, 10*1024*1024) // 10MB of data
	_, err := io.ReadFull(rand.Reader, content)
	assert.Nil(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		err := os.WriteFile(filePath, content, 0644)
		assert.Nil(t, err)

		readContent, err := os.ReadFile(filePath)
		assert.Nil(t, err)
		assert.Equal(t, content, readContent)
		assert.Equal(t, len(content), len(readContent))
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test Read Write From Same handle
func TestOpenWriteRead(t *testing.T) {
	t.Parallel()
	filename := "testfile_open_write_read.txt"
	tempbuffer := make([]byte, 4*1024)
	databuffer := make([]byte, 4*1024) // 4KB buffer
	_, err := io.ReadFull(rand.Reader, databuffer)
	assert.Nil(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.Nil(t, err)
		written, err := file.WriteAt(databuffer, 200)
		assert.Nil(t, err)
		assert.Equal(t, 4096, written)
		read, err := file.Read(tempbuffer)
		assert.Nil(t, err)
		assert.Equal(t, 4096, read)
		err = file.Close()
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)

}

// Test for writing from 1 fd and reading from another fd.
func TestOpenWriteReadMultipleHandles(t *testing.T) {
	t.Parallel()
	filename := "testfile_open_write_read_multiple_handles.txt"
	tempbuffer := make([]byte, 4*1024)
	databuffer := make([]byte, 4*1024) // 4KB buffer
	_, err := io.ReadFull(rand.Reader, databuffer)
	assert.Nil(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.Nil(t, err)
		file2, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.Nil(t, err)

		for range 10 { // Write the buffer 10 times from file
			written, err := file.Write(databuffer)
			assert.Nil(t, err)
			assert.Equal(t, written, 4*1024)
		}
		for range 10 { // Read the buffer 10 times
			read, err := file2.Read(tempbuffer)
			assert.Nil(t, err)
			assert.Equal(t, read, 4*1024)
			assert.Equal(t, databuffer, tempbuffer)
		}
		err = file.Close()
		assert.Nil(t, err)
		err = file2.Close()
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test rand sparse writing on a file.
func TestRandSparseWriting(t *testing.T) {
	t.Parallel()
	filename := "testfile_sparse_write.txt"
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.Nil(t, err)

		written, err := file.WriteAt([]byte("Hello"), 1024*1024) // Write at 1MB offset, 1st block
		assert.Nil(t, err)
		assert.Equal(t, written, 5)

		written, err = file.WriteAt([]byte("World"), 12*1024*1024) // Write at 12MB offset, 2nd block
		assert.Nil(t, err)
		assert.Equal(t, written, 5)

		written, err = file.WriteAt([]byte("Cosmos"), 30*1024*1024) // Write at 30MB offset, 4th block
		assert.Nil(t, err)
		assert.Equal(t, written, 6)

		err = file.Close()
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test sparse writing on blockoverlap assume block size as 8MB,
// write 4K buffers on overlapping zones of blocks.
func TestSparseWritingBlockOverlap(t *testing.T) {
	t.Parallel()
	filename := "testfile_block_overlap.txt"
	blockSize := 8 * 1024 * 1024 // 8MB
	bufferSize := 4 * 1024       // 4KB
	databuf := make([]byte, bufferSize)
	_, err := io.ReadFull(rand.Reader, databuf)
	assert.Nil(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.Nil(t, err)

		for i := 1; i <= 2; i++ {
			offset := i * blockSize
			offset -= 2 * 1024
			_, err = file.WriteAt(databuf, int64(offset))
			assert.Nil(t, err)
		}

		err = file.Close()
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test write at end of the file and call truncate to expand at the middle of the writes.
// Test write at end of the file and call truncate to shrink at the middle of the writes.
// Test open, shrink, write, close, This should result in hole at the middle
// Test open, expand, write at middle, close, This should change the file size.
// Test open, expand, write at end, close, This should change the file size.
// Test stripe writing with go routines.

// Test stripe writing.
// stripe writing means opening the files at different offsets and writing from that offset writing some data and finally close all the file descriptions.
func TestStripeWriting(t *testing.T) {
	t.Parallel()
	filename := "testfile_stripe_writing.txt"
	content := []byte("Stripe writing test data")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file0, err := os.Create(filePath)
		assert.Nil(t, err)
		file1, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.Nil(t, err)
		file2, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.Nil(t, err)

		written, err := file0.WriteAt(content, int64(0)) //write at 0MB
		assert.Nil(t, err)
		assert.Equal(t, len(content), written)
		written, err = file1.WriteAt(content, int64(8*1024*1024)) //write at 8MB
		assert.Nil(t, err)
		assert.Equal(t, len(content), written)
		written, err = file2.WriteAt(content, int64(16*1024*1024)) //write at 16MB
		assert.Nil(t, err)
		assert.Equal(t, len(content), written)

		err = file0.Close()
		assert.Nil(t, err)
		err = file1.Close()
		assert.Nil(t, err)
		err = file2.Close()
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test stripe writing with dup. same as the stripe writing but rather than opening so many files duplicate the file descriptor.
func TestStripeWritingWithDup(t *testing.T) {
	t.Parallel()
	filename := "testfile_stripe_writing_dup.txt"
	content := []byte("Stripe writing with dup test data")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.Nil(t, err)
		fd1, err := syscall.Dup(int(file.Fd()))
		assert.NotEqual(t, int(file.Fd()), fd1)
		assert.Nil(t, err)

		fd2, err := syscall.Dup(int(file.Fd()))
		assert.NotEqual(t, int(file.Fd()), fd1)
		assert.Nil(t, err)

		written, err := file.WriteAt(content, int64(0))
		assert.Nil(t, err)
		assert.Equal(t, len(content), written)
		written, err = syscall.Pwrite(fd1, content, int64(8*1024*1024))
		assert.Nil(t, err)
		assert.Equal(t, len(content), written)
		written, err = syscall.Pwrite(fd1, content, int64(16*1024*1024))
		assert.Nil(t, err)
		assert.Equal(t, len(content), written)

		err = file.Close()
		assert.Nil(t, err)
		err = syscall.Close(fd1)
		assert.Nil(t, err)
		err = syscall.Close(fd2)
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test stripe reading. Create a large file say 32M, then open the files at different offsets and whether data is getting matched.
func TestStripeReading(t *testing.T) {
	t.Parallel()
	filename := "testfile_stripe_reading.txt"
	content := []byte("Stripe Reading Test data")
	tempbuf := make([]byte, len(content))
	offsets := []int64{69, 8*1024*1024 + 69, 16*1024*1024 + 69}
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.Nil(t, err)
		// Write to the file.
		for _, off := range offsets {
			written, err := file.WriteAt(content, int64(off))
			assert.Nil(t, err)
			assert.Equal(t, len(content), written)
		}
		err = file.Close()
		assert.Nil(t, err)
		// Read from the different offsets using different file descriptions
		file0, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.Nil(t, err)
		file1, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.Nil(t, err)
		file2, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.Nil(t, err)

		bytesread, err := file0.ReadAt(tempbuf, offsets[0]) //read at 0MB
		assert.Nil(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)
		bytesread, err = file1.ReadAt(tempbuf, offsets[1]) //read at 8MB
		assert.Nil(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)
		bytesread, err = file2.ReadAt(tempbuf, offsets[2]) //read at 16MB
		assert.Nil(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)

		err = file0.Close()
		assert.Nil(t, err)
		err = file1.Close()
		assert.Nil(t, err)
		err = file2.Close()
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test stripe reading with dup.
func TestStripeReadingWithDup(t *testing.T) {
	t.Parallel()
	filename := "testfile_stripe_reading_dup.txt"
	content := []byte("Stripe Reading With Dup Test data")
	tempbuf := make([]byte, len(content))
	offsets := []int64{69, 8*1024*1024 + 69, 16*1024*1024 + 69}
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.Nil(t, err)
		// Write to the file.
		for _, off := range offsets {
			written, err := file.WriteAt(content, int64(off))
			assert.Nil(t, err)
			assert.Equal(t, len(content), written)
		}
		err = file.Close()
		assert.Nil(t, err)
		// Read from the different offsets using different file descriptions
		file0, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.Nil(t, err)
		fd1, err := syscall.Dup(int(file0.Fd()))
		assert.NotEqual(t, int(file.Fd()), fd1)
		assert.Nil(t, err)
		fd2, err := syscall.Dup(int(file0.Fd()))
		assert.NotEqual(t, int(file.Fd()), fd1)
		assert.Nil(t, err)

		bytesread, err := file0.ReadAt(tempbuf, offsets[0]) //read at 0MB
		assert.Nil(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)
		bytesread, err = syscall.Pread(fd1, tempbuf, offsets[1]) //write at 8MB
		assert.Nil(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)
		bytesread, err = syscall.Pread(fd2, tempbuf, offsets[2]) //write at 16MB
		assert.Nil(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)

		err = file0.Close()
		assert.Nil(t, err)
		err = syscall.Close(fd1)
		assert.Nil(t, err)
		err = syscall.Close(fd2)
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test O_TRUNC flag
func TestOTruncFlag(t *testing.T) {
	t.Parallel()
	filename := "testfile_trunc.txt"
	content := []byte("Hello, World!")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		err := os.WriteFile(filePath, content, 0644)
		assert.Nil(t, err)

		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC, 0644)
		assert.Nil(t, err)
		err = file.Close()
		assert.Nil(t, err)

		readContent, err := os.ReadFile(filePath)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(readContent))
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

func TestOTruncWhileWriting(t *testing.T) {
	t.Parallel()
	OTruncWhileWritingHelper(t, 64*1024)
	OTruncWhileWritingHelper(t, 10*1024*1024)
	OTruncWhileWritingHelper(t, 24*1024*1024)
}

func OTruncWhileWritingHelper(t *testing.T, size int) {
	filename := "testfile_O_trunc_while_writing.txt"
	databuf := make([]byte, 4096)
	_, err := io.ReadFull(rand.Reader, databuf)
	assert.Nil(t, err)
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)

		file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
		assert.Nil(t, err)

		for i := 0; i < size; i += 4096 {
			bytesWritten, err := file.Write(databuf)
			assert.Equal(t, 4096, bytesWritten)
			assert.Nil(t, err)
		}
		// lets open file with O_TRUNC
		file2, err := os.OpenFile(filePath, os.O_TRUNC, 0644)
		assert.Nil(t, err)

		// Continue the write on first fd.
		bytesWritten, err := file.Write(databuf)
		assert.Equal(t, 4096, bytesWritten)
		assert.Nil(t, err)
		// Now a big hole is formed at the starting of the file
		err = file2.Close()
		assert.Nil(t, err)
		err = file.Close()
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

func TestOTruncWhileReading(t *testing.T) {
	t.Parallel()
	OTruncWhileReadingHelper(t, 64*1024)
	OTruncWhileReadingHelper(t, 10*1024*1024)
	OTruncWhileReadingHelper(t, 24*1024*1024)
}
func OTruncWhileReadingHelper(t *testing.T, size int) {
	filename := "testfile_O_trunc_while_reading.txt"
	databuf := make([]byte, 4096)
	_, err := io.ReadFull(rand.Reader, databuf)
	assert.Nil(t, err)
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		// Create the file with desired size before starting the test
		file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
		assert.Nil(t, err)

		for i := 0; i < size; i += 4096 {
			bytesWritten, err := file.Write(databuf)
			assert.Equal(t, 4096, bytesWritten)
			assert.Nil(t, err)
		}
		err = file.Close()
		assert.Nil(t, err)
		//------------------------------------------------------
		// Start reading the file
		file, err = os.OpenFile(filePath, os.O_RDONLY, 0644)
		assert.Nil(t, err)
		bytesread, err := file.Read(databuf)
		assert.Equal(t, 4096, bytesread)
		assert.Nil(t, err)

		// lets open file with O_TRUNC
		file2, err := os.OpenFile(filePath, os.O_TRUNC, 0644)
		assert.Nil(t, err)

		// Continue the reading on first fd.
		bytesWritten, err := file.Read(databuf)
		assert.Equal(t, 0, bytesWritten)
		assert.Equal(t, io.EOF, err)

		err = file2.Close()
		assert.Nil(t, err)
		err = file.Close()
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test unlink on open
func TestUnlinkOnOpen(t *testing.T) {
	t.Parallel()
	filename := "testfile_unlink.txt"
	content := []byte("Hello, World!")
	content2 := []byte("Hello, Cosmos")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		//Open the file
		file, err := os.Create(filePath)
		assert.Nil(t, err)
		written, err := file.Write(content)
		assert.Equal(t, written, 13)
		assert.Nil(t, err)

		// Delete the file
		err = os.Remove(filePath)
		assert.Nil(t, err)
		// Read the content of the file after deleting the file.
		readContent := make([]byte, len(content))
		_, err = file.ReadAt(readContent, 0)
		assert.Nil(t, err)
		assert.Equal(t, string(content), string(readContent))

		err = file.Close()
		assert.Nil(t, err)

		// Open the file again
		_, err = os.Open(filePath)
		assert.NotNil(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "no such file or directory")
		}

		// Write to the file
		err = os.WriteFile(filePath, content2, 0644)
		assert.Nil(t, err)

		file2, err := os.Open(filePath)
		assert.Nil(t, err)

		// This read should be served from the newly created file
		_, err = file2.Read(readContent)
		assert.Nil(t, err)
		assert.Equal(t, string(content2), string(readContent))
	}
	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test for multiple handles, parallel flush calls while writing.

func TestParllelFlushCalls(t *testing.T) {
	t.Parallel()
	filename := "testfile_parallel_flush_calls.txt"
	databuffer := make([]byte, 4*1024) // 4KB buffer
	_, err := io.ReadFull(rand.Reader, databuffer)
	assert.Nil(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file0, err := os.Create(filePath)
		assert.Nil(t, err)
		file1, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.Nil(t, err)

		// for each 1MB writes trigger a flush call from another go routine.
		trigger_flush := make(chan struct{}, 1)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				_, ok := <-trigger_flush
				if !ok {
					break
				}
				err := file1.Sync()
				assert.Nil(t, err)
				if err != nil {
					fmt.Printf("%s", err.Error())
				}
			}
		}()
		// Write 40M data
		for i := 0; i < 40*1024*1024; i += 4 * 1024 {
			if i%(1*1024*1024) == 0 {
				trigger_flush <- struct{}{}
			}
			byteswritten, err := file0.Write(databuffer)
			assert.Equal(t, 4*1024, byteswritten)
			assert.Nil(t, err)
		}
		close(trigger_flush)
		wg.Wait()
		err = file0.Close()
		assert.Nil(t, err)
		err = file1.Close()
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Dup the FD and do parllel flush calls while writing.
func TestParllelFlushCallsByDuping(t *testing.T) {
	filename := "testfile_parallel_flush_calls_using_dup.txt"
	databuffer := make([]byte, 4*1024) // 4KB buffer
	_, err := io.ReadFull(rand.Reader, databuffer)
	assert.Nil(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.Nil(t, err)

		fd1, err := syscall.Dup(int(file.Fd()))
		assert.NotEqual(t, int(file.Fd()), fd1)
		assert.Nil(t, err)

		// for each 1MB writes trigger a flush call from another go routine.
		trigger_flush := make(chan struct{}, 1)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				_, ok := <-trigger_flush
				if !ok {
					break
				}
				err := syscall.Fdatasync(fd1)
				assert.Nil(t, err)
			}
		}()
		// Write 40M data
		for i := 0; i < 40*1024*1024; i += 4 * 1024 {
			if i%(1*1024*1024) == 0 {
				trigger_flush <- struct{}{}
			}
			byteswritten, err := file.Write(databuffer)
			assert.Equal(t, 4*1024, byteswritten)
			assert.Nil(t, err)
		}
		close(trigger_flush)
		wg.Wait()
		err = file.Close()
		assert.Nil(t, err)
		err = syscall.Close(fd1)
		assert.Nil(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Aggressive random write on large file.

func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		usr, err := user.Current()
		if err != nil {
			return "", err
		}
		path = filepath.Join(usr.HomeDir, path[2:])
	}
	return filepath.Abs(path)
}

func TestMain(m *testing.M) {
	mountpointsFlag := flag.String("mountpoints", "", "Comma-separated list of mountpoints")
	flag.Parse()

	if *mountpointsFlag != "" {
		mountpoints = strings.Split(*mountpointsFlag, ",")
		for i, mnt := range mountpoints {
			absPath, err := expandPath(mnt)
			if err != nil {
				panic(err)
			}
			mountpoints[i] = absPath
		}
	}

	os.Exit(m.Run())
}
