package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func giveFormatFilterObj(singleFilter string, thisFilter string, filterMap map[string]filterCreator) (Filter, bool) {
	singleFilter = strings.Map(StringConv, singleFilter)
	if (len(singleFilter) <= len(thisFilter)+1) || (singleFilter[len(thisFilter)] != '=') || (!(singleFilter[len(thisFilter)+1] >= 'a' && singleFilter[len(thisFilter)+1] <= 'z')) {
		return nil, false
	}
	value := singleFilter[len(thisFilter)+1:]
	return filterMap[thisFilter](value), true
}
