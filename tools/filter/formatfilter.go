package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type FormatFilter struct { //formatFilter and its attributes
	ext_type string
}

func (filter FormatFilter) Apply(fileInfo os.FileInfo) bool { //Apply fucntion for format filter , check wheather a file passes the constraints
	fmt.Println("Format Filter ", filter, " file name ", fileInfo.Name())
	fileExt := filepath.Ext(fileInfo.Name())
	chkstr := "." + filter.ext_type
	return chkstr == fileExt
}

func newFormatFilter(args ...interface{}) Filter { // used for dynamic creation of formatFilter using map
	return FormatFilter{
		ext_type: args[0].(string),
	}
}
