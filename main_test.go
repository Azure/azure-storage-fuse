// +build !unittest

package main

import (
	"os"
	"strings"
	"testing"
)

func TestMain(t *testing.T) {
	var args []string
	for _, arg := range os.Args {
		if !strings.HasPrefix(arg, "-test") {
			args = append(args, arg)
		}
	}
	os.Args = args
	if strings.Contains(os.Args[0], "blobfuse2.test") {
		t.Log("Starting coverage test")
		main()
	} else {
		t.Error("Failed to start blobfuse2 binary")
	}
}
