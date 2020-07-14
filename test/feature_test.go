package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

var mntPath string = ""
var adlsTest bool = false

var minBuff = make([]byte, 1024)
var medBuff = make([]byte, (10 * 1024 * 1024))
var hugeBuff = make([]byte, (500 * 1024 * 1024))

// -------------- Directory Level Testings -------------------

// # Create Directory with a simple name
func TestDirCreateSimple(t *testing.T) {
	dirName := mntPath + "/test1"
	err := os.Mkdir(dirName, 0777)
	if err != nil {
		t.Errorf("Failed to create directory : " + dirName + "(" + err.Error() + ")")
	}
}

// # Create Directory that already exists
func TestDirCreateDuplicate(t *testing.T) {
	dirName := mntPath + "/test1"
	err := os.Mkdir(dirName, 0777)
	if err != nil {
		if !strings.Contains(err.Error(), "file exists") {
			t.Errorf("Failed to create directory : " + dirName + "(" + err.Error() + ")")
		}
	}
}

// # Create Directory with special characters in name
func TestDirCreateSplChar(t *testing.T) {
	if adlsTest == true {
		//dirName := mntPath + "/!@#$%^&*()_+=-{}[]|?><.,\`~"
		dirName := mntPath + "/" + "@#$^&*()_+=-{}[]|?><.,~"
		err := os.Mkdir(dirName, 0777)
		if err != nil {
			t.Errorf("Failed to create directory : " + dirName + "(" + err.Error() + ")")
		}
	} else {
		t.Logf("Ignoring this case as ADLS is not configued")
	}
}

// # Rename a directory
func TestDirRename(t *testing.T) {
	dirName := mntPath + "/test1"
	newName := mntPath + "/test1_new"
	err := os.Rename(dirName, newName)
	if err != nil {
		t.Errorf("Failed to rename directory to : " + newName + "(" + err.Error() + ")")
	}

	//  Deleted directory shall not be present in the contianer now
	if _, err = os.Stat(dirName); !os.IsNotExist(err) {
		t.Errorf("Old directory exists even after rename : " + dirName + "(" + err.Error() + ")")
	}
}

// # Move an empty directory
func TestDirMoveEmpty(t *testing.T) {
	dir2Name := mntPath + "/test2"
	err := os.Mkdir(dir2Name, 0777)
	if err != nil {
		t.Errorf("Failed to create directory : " + dir2Name + "(" + err.Error() + ")")
	}

	dir3Name := mntPath + "/test3"
	err = os.Mkdir(dir3Name, 0777)
	if err != nil {
		t.Errorf("Failed to create directory : " + dir3Name + "(" + err.Error() + ")")
	}

	err = os.Rename(dir2Name, dir3Name+"/test2")
	if err != nil {
		t.Errorf("Failed to Move directory : " + dir2Name + "(" + err.Error() + ")")
	}
}

// # Move an non-empty directory
func TestDirMoveNonEmpty(t *testing.T) {
	dir2Name := mntPath + "/test2NE"
	err := os.Mkdir(dir2Name, 0777)
	if err != nil {
		t.Errorf("Failed to create directory : " + dir2Name + "(" + err.Error() + ")")
	}

	file1Name := dir2Name + "/test.txt"
	_, err = os.Create(file1Name)
	if err != nil {
		t.Errorf("Failed to create file : " + file1Name + "(" + err.Error() + ")")
	}

	dir3Name := mntPath + "/test3NE"
	err = os.Mkdir(dir3Name, 0777)
	if err != nil {
		t.Errorf("Failed to create directory : " + dir3Name + "(" + err.Error() + ")")
	}

	err = os.Rename(dir2Name, dir3Name+"/test2")
	if err != nil {
		t.Errorf("Failed to Move directory : " + dir2Name + "(" + err.Error() + ")")
	}
}

// # Delete non-empty directory
func TestDirDeleteEmpty(t *testing.T) {
	dirName := mntPath + "/test1_new"
	err := os.Remove(dirName)
	if err != nil {
		t.Errorf("Failed to delete directory : " + dirName + "(" + err.Error() + ")")
	}
}

// # Delete non-empty directory
func TestDirDeleteNonEmpty(t *testing.T) {
	dirName := mntPath + "/test3NE"
	err := os.Remove(dirName)
	if err != nil {
		if !strings.Contains(err.Error(), "directory not empty") {
			t.Errorf("Failed to delete directory : " + dirName + "(" + err.Error() + ")")
		}
	}
}

// # Delete non-empty directory recursively
func TestDirDeleteRecursive(t *testing.T) {
	dirName := mntPath + "/test3NE"
	err := os.RemoveAll(dirName)
	if err != nil {
		t.Errorf("Failed to delete directory recursively : " + dirName + "(" + err.Error() + ")")
	}
}

// # Get stats of a directory
func TestDirGetStats(t *testing.T) {
	dirName := mntPath + "/test3"
	stat, err := os.Stat(dirName)
	if err != nil {
		t.Errorf("Failed to get stats of directory : " + dirName + "(" + err.Error() + ")")
	}
	modTineDiff := time.Now().Sub(stat.ModTime())
	fmt.Println(stat.ModTime())
	//fmt.Println(stat.ModTime().Unix())

	// for directory block blob may still return timestamp as 0
	// So compare the time only if epoch is non-zero
	if stat.ModTime().Unix() != 0 {
		if stat.IsDir() != true ||
			stat.Name() != "test3" ||
			modTineDiff.Hours() > 1 {
			t.Errorf("Invalid Stats of directory : " + dirName)
		}
	}
}

// # Change mod of directory
func TestDirChmod(t *testing.T) {
	if adlsTest == true {
		dirName := mntPath + "/test3"
		err := os.Chmod(dirName, 0744)
		if err != nil {
			t.Errorf("Failed to change permissoin of directory : " + dirName + "(" + err.Error() + ")")
		}
		err = nil
		stat, err := os.Stat(dirName)
		if err != nil {
			t.Errorf("Failed to get stats of directory : " + dirName + "(" + err.Error() + ")")
		}
		if stat.Mode().Perm() != 0744 {
			t.Errorf("Failed to modify permissions of directory : " + dirName)
		}
	} else {
		t.Logf("Ignoring this case as ADLS is not configued")
	}
}

// # List directory
func TestDirList(t *testing.T) {
	dirName := mntPath
	files, err := ioutil.ReadDir(mntPath)
	if err != nil ||
		len(files) < 1 {
		t.Errorf("Failed to list directory : " + dirName + "(" + err.Error() + ")")
	}
	/*for _, file := range files {
		fmt.Println(file.Name())
	}*/
}

// # List directory recursively
func TestDirListRecursive(t *testing.T) {
	dirName := mntPath
	var files []string
	err := filepath.Walk(mntPath, func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	})
	if err != nil ||
		len(files) < 1 {
		t.Errorf("Failed to list directory : " + dirName)
	}
	//fmt.Print(files)
}

// # LASTTEST : Clean up everything created by Dir Test
func TestDirClean(t *testing.T) {
	err := os.RemoveAll(mntPath + "/")
	if err != nil {
		t.Errorf("Failed to cleanup (" + err.Error() + ")")
	}

	err = os.Mkdir(mntPath, 0777)
	if err != nil &&
		!strings.Contains(err.Error(), "file exists") {
		t.Errorf("Failed to re-create (" + err.Error() + ")")
	}
}

// -------------- File Level Testings -------------------

// # Create file test
func TestFileCreate(t *testing.T) {
	fileName := mntPath + "/small_write.txt"

	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	if err != nil {
		t.Errorf("Failed to create file " + fileName + " (" + err.Error() + ")")
	}
	srcFile.Close()
}

func TestFileCreateSpclChar(t *testing.T) {
	fileName := mntPath + "/ΣΑΠΦΩ.txt"

	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	if err != nil {
		t.Errorf("Failed to create file " + fileName + " (" + err.Error() + ")")
	}
	srcFile.Close()
}

func TestFileCreateUtf8Char(t *testing.T) {
	fileName := mntPath + "/भारत.txt"

	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	if err != nil {
		t.Errorf("Failed to create file " + fileName + " (" + err.Error() + ")")
	}
	srcFile.Close()
}

// # Write a small file
func TestFileWriteSmall(t *testing.T) {
	fileName := mntPath + "/small_write.txt"
	err := ioutil.WriteFile(fileName, minBuff, 0777)
	if err != nil {
		t.Errorf("Failed to write file " + fileName + " (" + err.Error() + ")")
	}
}

// # Read a small file
func TestFileReadSmall(t *testing.T) {
	fileName := mntPath + "/small_write.txt"
	data, err := ioutil.ReadFile(fileName)
	if err != nil ||
		len(data) != len(minBuff) {
		fmt.Println(len(data))
		fmt.Println(len(minBuff))
		t.Errorf("Failed to Read file " + fileName + " (" + err.Error() + ")")
	}
}

// # Create duplicate file
func TestFileCreateDuplicate(t *testing.T) {
	fileName := mntPath + "/small_write.txt"
	_, err := os.Create(fileName)
	if err != nil {
		if !strings.Contains(err.Error(), "file exists") {
			t.Errorf("Failed to create file : " + fileName + "(" + err.Error() + ")")
		}
	}
}

// # Truncate a file
func TestFileTruncate(t *testing.T) {
	fileName := mntPath + "/small_write.txt"
	err := os.Truncate(fileName, 2)
	if err != nil {
		t.Errorf("Failed to Truncate file " + fileName + " (" + err.Error() + ")")
	}

	data, err := ioutil.ReadFile(fileName)
	fmt.Println(len(data))
	if err != nil ||
		len(data) > 2 {
		t.Errorf("Failed to Read file " + fileName + " (" + err.Error() + ")")
	}
}

// # Create file matching directory name
func TestFileNameConflict(t *testing.T) {
	dirName := mntPath + "/test123"
	fileName := mntPath + "/test"

	err := os.Mkdir(dirName, 0777)
	if err != nil {
		t.Errorf("Failed to create directory " + dirName + " (" + err.Error() + ")")
	}

	_, err = os.Create(fileName)
	if err != nil {
		t.Errorf("Failed to create file " + fileName + " (" + err.Error() + ")")
	}
	time.Sleep(3)
}

// # Copy file from once directory to another
func TestFileCopy(t *testing.T) {
	dirName := mntPath + "/test123"
	fileName := mntPath + "/test"
	dstFileName := dirName + "/test_copy.txt"

	srcFile, err := os.OpenFile(fileName, os.O_RDWR, 0777)
	defer srcFile.Close()

	dstFile, err := os.Create(dstFileName)
	defer dstFile.Close()

	_, err = io.Copy(srcFile, dstFile)
	if err != nil {
		t.Errorf("Failed to copy file " + dstFileName + " (" + err.Error() + ")")
	}
}

// # Get stats of a file
func TestFileGetStat(t *testing.T) {
	fileName := mntPath + "/test"
	stat, err := os.Stat(fileName)
	if err != nil {
		t.Errorf("Failed to get stats of directory : " + fileName + "(" + err.Error() + ")")
	}
	modTineDiff := time.Now().Sub(stat.ModTime())
	fmt.Println(stat.ModTime())

	if stat.IsDir() == true ||
		stat.Name() != "test" ||
		modTineDiff.Hours() > 1 {
		t.Errorf("Invalid Stats of file : " + fileName)
	}
}

// # Change mod of file
func TestFileChmod(t *testing.T) {
	if adlsTest {
		fileName := mntPath + "/test"
		err := os.Chmod(fileName, 0744)
		if err != nil {
			t.Errorf("Failed to change permissoin of file : " + fileName + "(" + err.Error() + ")")
		}
		stat, err := os.Stat(fileName)
		if err != nil {
			t.Errorf("Failed to get stats of file : " + fileName + "(" + err.Error() + ")")
		}
		if stat.Mode().Perm() != 0744 {
			t.Errorf("Failed to modify permissions of directory : " + fileName)
		}
	} else {
		t.Logf("Ignoring this case as ADLS is not configued")
	}
}

// # Create multiple med files
func TestFileCreateMulti(t *testing.T) {
	fileName := mntPath + "/multi"

	for i := 0; i < 10; i++ {
		newFile := fileName + strconv.Itoa(i)
		err := ioutil.WriteFile(newFile, medBuff, 0777)
		if err != nil {
			t.Errorf("Failed to create file " + newFile + " (" + err.Error() + ")")
		}
	}

}

// # Delete single files
func TestFileDeleteSingle(t *testing.T) {
	fileName := mntPath + "/multi0"
	if err := os.Remove(fileName); err != nil {
		t.Errorf("Failed to delete file " + fileName + " (" + err.Error() + ")")
	}
}

// # Delete multiple files
func TestFileDeleteMulti(t *testing.T) {
	fileName := mntPath + "/multi*"
	files, err := filepath.Glob(fileName)
	if err != nil {
		t.Errorf("Failed to get file list " + fileName + " (" + err.Error() + ")")
	}

	for _, f := range files {
		if err = os.Remove(f); err != nil {
			t.Errorf("Failed to delete file " + f + " (" + err.Error() + ")")
		}
	}
}

// # LASTTEST : Clean up everything created by Dir Test
func TestFileClean(t *testing.T) {
	err := os.RemoveAll(mntPath + "/")
	if err != nil {
		t.Errorf("Failed to cleanup (" + err.Error() + ")")
	}

	err = os.Mkdir(mntPath, 0777)
	if err != nil &&
		!strings.Contains(err.Error(), "file exists") {
		t.Errorf("Failed to re-create (" + err.Error() + ")")
	}
}

// -------------- SymLink Level Testings -------------------

// # Create a symlink to a file
func TestLinkCreate(t *testing.T) {
	fileName := mntPath + "/small_write1.txt"
	symName := mntPath + "/small.lnk"
	err := ioutil.WriteFile(fileName, minBuff, 0777)
	if err != nil {
		t.Errorf("Failed to write file " + fileName + " (" + err.Error() + ")")
	}

	err = os.Symlink(fileName, symName)
	if err != nil {
		t.Errorf("Failed to create symlink " + symName + " (" + err.Error() + ")")
	}
	time.Sleep(3)
}

// # Read a small file using symlink
func TestLinkRead(t *testing.T) {
	fileName := mntPath + "/small.lnk"
	data, err := ioutil.ReadFile(fileName)
	if err != nil ||
		len(data) != len(minBuff) {
		fmt.Println(len(data))
		fmt.Println(len(minBuff))
		t.Errorf("Failed to Read symlink " + fileName)
	}
}

// # Write a small file using symlink
func TestLinkWrite(t *testing.T) {
	fileName := mntPath + "/small.lnk"
	targetName := mntPath + "/small_write1.txt"
	err := ioutil.WriteFile(fileName, medBuff, 0777)
	if err != nil {
		t.Errorf("Failed to write file " + fileName + " (" + err.Error() + ")")
	}

	stat, err := os.Stat(targetName)
	modTineDiff := time.Now().Sub(stat.ModTime())
	fmt.Println(stat.ModTime())
	if modTineDiff.Minutes() > 2 {
		t.Errorf("Last modified time mismatch for " + targetName)
	}
}

// # Rename the target file and validate read on symlink fails
func TestLinkRenameTarget(t *testing.T) {
	fileName := mntPath + "/small_write1.txt"
	symName := mntPath + "/small.lnk"
	fileNameNew := mntPath + "/small_write_new.txt"
	err := os.Rename(fileName, fileNameNew)
	if err != nil {
		t.Errorf("Failed to rename target file to : " + fileNameNew + "(" + err.Error() + ")")
	}

	_, err = ioutil.ReadFile(symName)
	if err == nil {
		t.Errorf("Failed to read using symlink, target deleted :  " + fileName + "(" + err.Error() + ")")
	}
	err = os.Rename(fileNameNew, fileName)
}

// # Delete the symklink and check target file is still intact
func TestLinkDeleteReadTarget(t *testing.T) {
	fileName := mntPath + "/small_write1.txt"
	symName := mntPath + "/small.lnk"

	err := os.Remove(symName)
	if err != nil {
		t.Errorf("Failed to delete symlink : " + symName + "(" + err.Error() + ")")
	}

	data, err := ioutil.ReadFile(fileName)
	if err != nil ||
		len(data) != len(medBuff) {
		t.Errorf("Failed to read using symlink, target deleted :  " + fileName + "(" + err.Error() + ")")
	}

	err = os.Symlink(fileName, symName)
	if err != nil {
		t.Errorf("Failed to create symlink " + symName + " (" + err.Error() + ")")
	}
}

// # Delete a symlink to a file
func TestLinkDelete(t *testing.T) {
	/*symName := mntPath + "/small.lnk"
	err := os.Remove(symName)
	if err != nil {
		t.Errorf("Failed to delete symlink " + symName + " (" + err.Error() + ")")
	}*/
}

// -------------- Main Method to start the Testing -------------------

func TestMain(m *testing.M) {
	// Get the mount path from command line argument
	pathPtr := flag.String("mnt-path", ".", "Mount Path of Container")
	adlsPtr := flag.String("adls", "false", "Account is ADLS or not")
	flag.Parse()

	// Create directory for testing the feature test on mount path
	mntPath = *pathPtr + "/feature"
	if *adlsPtr == "true" {
		fmt.Println("ADLS Testing...")
		adlsTest = true
	} else {
		fmt.Println("BLOCK Blob Testing...")
	}

	os.Mkdir(mntPath, 0777)

	rand.Read(minBuff)
	rand.Read(medBuff)
	rand.Read(hugeBuff)

	// Run the actual feature test
	m.Run()

	//  Wipe out the test directory created for feature test
	os.RemoveAll(*pathPtr)
}
