// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package scenarios

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileCreate(t *testing.T) {
	t.Parallel()
	filename := "testfile_create.txt"
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)
		err = file.Close()
		assert.NoError(t, err)

		_, err = os.Stat(filePath)
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// TODO: Test O_EXCL flag once supported
