package scenarios

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
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

func TestFileTruncateExpand(t *testing.T) {
	t.Parallel()
	filename := "testfile_truncate_expand.txt"
	FileTruncate(t, filename, 5, 20, truncate)
	FileTruncate(t, filename, 5, 20*1024*1024, truncate)
	FileTruncate(t, filename, 20*1024*1024, 30*1024*1024, truncate)
}

func TestFileTruncateShrink(t *testing.T) {
	t.Parallel()
	filename := "testfile_truncate_shrink.txt"
	FileTruncate(t, filename, 20, 5, truncate)
	FileTruncate(t, filename, 20*1024*1024, 5, truncate)
	FileTruncate(t, filename, 30*1024*1024, 20*1024*1024, truncate)
}

// tests for truncate function which works on handle (i.e ftruncate)

func TestFileFtruncateExpand(t *testing.T) {
	t.Parallel()
	filename := "testfile_ftruncate_expand.txt"
	FileTruncate(t, filename, 5, 20, ftruncate)
	FileTruncate(t, filename, 5, 20*1024*1024, ftruncate)
	FileTruncate(t, filename, 20*1024*1024, 30*1024*1024, ftruncate)
}

func TestFileFtruncateShrink(t *testing.T) {
	t.Parallel()
	filename := "testfile_ftruncate_shrink.txt"
	FileTruncate(t, filename, 20, 5, ftruncate)
	FileTruncate(t, filename, 20*1024*1024, 5, ftruncate)
	FileTruncate(t, filename, 30*1024*1024, 20*1024*1024, ftruncate)
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

// Test for writing from 1 fd and reading from another fd.
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
		file2, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.Nil(t, err)

		for i := 0; i < 10; i++ { // Write the buffer 10 times from file
			written, err := file.Write(databuffer)
			assert.Nil(t, err)
			assert.Equal(t, written, 4*1024)
		}
		for i := 0; i < 10; i++ { // Read the buffer 10 times
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

		written, err := file0.WriteAt(content, int64(0)) //writ at 0MB
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

		bytesread, err := file0.ReadAt(tempbuf, offsets[0]) //writ at 0MB
		assert.Nil(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)
		bytesread, err = file1.ReadAt(tempbuf, offsets[1]) //write at 8MB
		assert.Nil(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)
		bytesread, err = file2.ReadAt(tempbuf, offsets[2]) //write at 16MB
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

		bytesread, err := file0.ReadAt(tempbuf, offsets[0]) //writ at 0MB
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

// Test unlink on open
func TestUnlinkOnOpen(t *testing.T) {
	t.Parallel()
	filename := "testfile_unlink.txt"
	content := []byte("Hello, World!")
	content2 := []byte("Hello, Cosmos")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		err := os.WriteFile(filePath, content, 0644)
		assert.Nil(t, err)
		//Open the file
		file, err := os.Open(filePath)
		assert.Nil(t, err)
		// Delete the file
		err = os.Remove(filePath)
		assert.Nil(t, err)
		// Open the file again
		_, err = os.Open(filePath)
		assert.Contains(t, err.Error(), "no such file or directory")
		// Write to the file
		err = os.WriteFile(filePath, content2, 0644)
		assert.Nil(t, err)

		file2, err := os.Open(filePath)
		assert.Nil(t, err)
		// This read should be served from the delted file
		readContent := make([]byte, len(content))
		_, err = file.Read(readContent)
		assert.Nil(t, err)
		assert.Equal(t, string(content), string(readContent))

		// This read should be served from the newly created file
		_, err = file2.Read(readContent)
		assert.Nil(t, err)
		assert.Equal(t, string(content2), string(readContent))

		err = file.Close()
		assert.Nil(t, err)
	}
}

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
