// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package scenarios

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileOpen(t *testing.T) {
	t.Parallel()
	filename := "testfile_open.txt"
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)
		err = file.Close()
		assert.NoError(t, err)

		file, err = os.Open(filePath)
		assert.NoError(t, err)
		err = file.Close()
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}
