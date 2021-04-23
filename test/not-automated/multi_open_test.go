package main

import (
	"crypto/md5"
	"encoding/hex"
	"flag"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"
)

var mntPath string = ""
var fileNameSuffix = "/testfile_1.txt"
var MD5Sum string = ""
var lastModTime time.Time
var fileSize = int64(20 * 1024 * 1024)
var medBuff = make([]byte, fileSize)

func GetFileHash(filePath string) (string, error) {
	var md5sum string
	file, err := os.Open(filePath)
	if err != nil {
		return md5sum, err
	}
	defer file.Close()
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return md5sum, err
	}
	hashInBytes := hash.Sum(nil)[:16]
	md5sum = hex.EncodeToString(hashInBytes)
	return md5sum, nil

}

func TestFileCreate(t *testing.T) {
	fileName := mntPath + fileNameSuffix

	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	if err != nil {
		t.Errorf("Failed to create file " + fileName + " (" + err.Error() + ")")
	}
	srcFile.Close()

	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)
	r.Read(medBuff)
	err = ioutil.WriteFile(fileName, medBuff, 0777)
	if err != nil {
		t.Errorf("Failed to write file " + fileName + " (" + err.Error() + ")")
	}
}

func TestFileOpen(t *testing.T) {
	fileName := mntPath + fileNameSuffix

	srcFile, err := os.OpenFile(fileName, os.O_RDWR, 0777)
	if err != nil {
		t.Errorf("Failed to create file " + fileName + " (" + err.Error() + ")")
	}
	defer srcFile.Close()

	stat, err := os.Stat(fileName)
	if err != nil {
		t.Errorf("Failed to get stat of file " + fileName + " (" + err.Error() + ")")
	}

	lastModTime = stat.ModTime()
	t.Logf("File size : %d, last modified time : %s", stat.Size(), lastModTime.String())

	MD5Sum, err = GetFileHash(fileName)
	if err != nil {
		t.Errorf("Failed to generate md5 (" + err.Error() + ")")
	}
	t.Logf("File Hash is '%s'", MD5Sum)
}

func ParallelOpenRW(wg *sync.WaitGroup, id int, t *testing.T) {
	defer wg.Done()
	fileName := mntPath + fileNameSuffix

	//t.Logf("Starting parallel open RW : %d", id)

	for {
		stat, err := os.Stat(fileName)
		if err != nil {
			t.Errorf("Failed to get stat of file " + fileName + " (" + err.Error() + ")")
		}

		if stat.Size() != int64(fileSize) {
			t.Logf("Rw %d Failed to validate stat of file "+fileName+" size : %d(%d), mod time %s(%s)", id, stat.Size(), fileSize, lastModTime.String(), stat.ModTime().String())
			time.Sleep(1 * time.Microsecond)
		} else {
			break
		}
	}

	srcFile, err := os.OpenFile(fileName, os.O_RDWR, 0777)
	if err != nil {
		t.Errorf("Failed to open file " + fileName + " (" + err.Error() + ")")
	}
	//time.Sleep(2 * time.Second)
	srcFile.Close()
}

func ParallelOpenR(wg *sync.WaitGroup, id int, t *testing.T) {
	defer wg.Done()
	fileName := mntPath + fileNameSuffix
	buf := make([]byte, 64)

	//t.Logf("Starting parallel open RD : %d", id)

	for {
		stat, err := os.Stat(fileName)
		if err != nil {
			t.Errorf("Failed to get stat of file " + fileName + " (" + err.Error() + ")")
		}

		if stat.Size() != int64(fileSize) {
			t.Logf("RD %d Failed to validate stat of file "+fileName+" size : %d(%d), mod time %s(%s)", id, stat.Size(), fileSize, lastModTime.String(), stat.ModTime().String())
			time.Sleep(1 * time.Microsecond)
		} else {
			break
		}
	}

	srcFile, err := os.OpenFile(fileName, os.O_RDONLY, 0777)
	if err != nil {
		t.Errorf("Failed to open file " + fileName + " (" + err.Error() + ")")
	}

	_, err = srcFile.Read(buf)
	if err != nil {
		t.Errorf("Failed to read 16 bytes from file ##############")
	}

	//time.Sleep(2 * time.Second)
	srcFile.Close()
}

func TestFileOpenParallel(t *testing.T) {
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go ParallelOpenRW(&wg, i, t)
		wg.Add(1)
		go ParallelOpenR(&wg, i, t)
		time.Sleep(1 * time.Second)
	}
	wg.Wait()
	t.Logf("All workers are done.")
}

func TestFileValidate(t *testing.T) {
	// Sleep for 5 seconds so that cache is evicted by then
	time.Sleep(5 * time.Second)
	fileName := mntPath + fileNameSuffix

	mdsum, _ := GetFileHash(fileName)

	stat, err := os.Stat(fileName)
	if err != nil {
		t.Errorf("Failed to get stat of file " + fileName + " (" + err.Error() + ")")
	}
	modtime := stat.ModTime()

	if modtime != lastModTime || mdsum != MD5Sum {
		t.Errorf("File stats have changed modtime %s(%s), mdsum %s(%s)", modtime.String(), lastModTime.String(), mdsum, MD5Sum)
	}
}

func TestMain(m *testing.M) {
	// Get the mount path from command line argument
	pathPtr := flag.String("mnt-path", ".", "Mount Path of Container")
	flag.Parse()

	// Create directory for testing the feature test on mount path
	mntPath = *pathPtr + "/parallel"
	os.Mkdir(mntPath, 0777)

	// Run the actual feature test
	m.Run()

	//  Wipe out the test directory created for feature test
	os.RemoveAll(mntPath)
}
