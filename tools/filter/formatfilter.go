package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type FormatFilter struct {
	ext_type string
}

func (filter FormatFilter) Apply(fileInfo os.FileInfo) bool {
	fmt.Println("FormatFilter called")
	fmt.Println("At this point data is ", filter, " file name ", fileInfo.Name())
	fileExt := filepath.Ext(fileInfo.Name())
	chkstr := "." + filter.ext_type
	return chkstr == fileExt
}

func newFormatFilter(args ...interface{}) Filter {
	return FormatFilter{
		ext_type: args[0].(string),
	}
}
