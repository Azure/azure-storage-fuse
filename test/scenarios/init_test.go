// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package scenarios

import (
	"crypto/md5"
	"encoding/hex"
	"flag"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Specify Mountpoints to check the file integrity across filesystems.
// Specifying one Mountpoint will check all the files for the errors.
var mountpoints []string
var directIOEnabledOnMountpoint bool

func calculateMD5(t *testing.T, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() {
		err := file.Close()
		assert.NoError(t, err)
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
			assert.NoError(t, err)
			md5sum, err := calculateMD5(t, filePath)
			assert.NoError(t, err)

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
		assert.NoError(t, err)
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
	// parse direct-io if enabled for mountpoint
	directIOFlag := flag.Bool("mount-point-direct-io", false, "is direct I/O enabled for mountpoint?")

	flag.Parse()

	if *directIOFlag {
		directIOEnabledOnMountpoint = true
	}

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
