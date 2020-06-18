package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var mntPath string = ""

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
	dirName := mntPath + "/!@#$%^&*()_+=-{}[]|?><.,`~"
	err := os.Mkdir(dirName, 0777)
	if err != nil {
		t.Errorf("Failed to create directory : " + dirName + "(" + err.Error() + ")")
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

	if stat.IsDir() != true ||
		stat.Name() != "test3" ||
		modTineDiff.Hours() > 1 {
		t.Errorf("Invalid Stats of directory : " + dirName)
	}
}

// # Change mod of directory
func TestDirChmod(t *testing.T) {
	dirName := mntPath + "/test3"
	err := os.Chmod(dirName, 0744)
	if err != nil {
		t.Errorf("Failed to change permissoin of directory : " + dirName + "(" + err.Error() + ")")
	}
	stat, err := os.Stat(dirName)
	if err != nil {
		t.Errorf("Failed to get stats of directory : " + dirName + "(" + err.Error() + ")")
	}
	if stat.Mode().Perm() != 0744 {
		t.Errorf("Failed to modify permissions of directory : " + dirName)
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
	err := os.RemoveAll(mntPath)
	if err != nil {
		t.Errorf("Failed to cleanup (" + err.Error() + ")")
	}
}

// -------------- File Level Testings -------------------

// -------------- SymLink Level Testings -------------------

// -------------- Main Method to start the Testing -------------------

func TestMain(m *testing.M) {
	// Get the mount path from command line argument
	pathPtr := flag.String("mnt-path", ".", "Mount Path")
	flag.Parse()

	// Create directory for testing the feature test on mount path
	mntPath = *pathPtr + "/feature"
	os.Mkdir(mntPath, 0777)

	// Run the actual feature test
	m.Run()

	//  Wipe out the test directory created for feature test
	//os.RemoveAll(mntPath)
}
