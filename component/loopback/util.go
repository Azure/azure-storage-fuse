package loopback

import (
	"errors"
	"os"
	"path/filepath"
)

func removeAllFilesWithGivenPrefix(prefix string) error {
	pattern := prefix + "*"

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, file := range matches {
		err1 := os.Remove(file)
		if err1 != nil {
			errors.Join(err, err1)
		}
	}
	return err
}
