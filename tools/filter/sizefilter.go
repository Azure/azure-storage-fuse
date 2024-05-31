package main

import (
	"fmt"
	"os"
)

type SizeFilter struct {
	less_than    float64
	greater_than float64
	equal_to     float64
}

func (filter SizeFilter) Apply(fileInfo os.FileInfo) bool {
	fmt.Println("size filter called")
	fmt.Println("At this point data is ", filter, " file name ", fileInfo.Name())
	if (filter.less_than != -1) && (filter.equal_to != -1) && (fileInfo.Size() <= int64(filter.less_than)) {
		return true
	} else if (filter.greater_than != -1) && (filter.equal_to != -1) && (fileInfo.Size() >= int64(filter.greater_than)) {
		return true
	} else if (filter.greater_than != -1) && (fileInfo.Size() > int64(filter.greater_than)) {
		return true
	} else if (filter.less_than != -1) && (fileInfo.Size() < int64(filter.less_than)) {
		return true
	} else if (filter.equal_to != -1) && (fileInfo.Size() == int64(filter.equal_to)) {
		return true
	}
	return false
}

func newSizeFilter(args ...interface{}) Filter {
	return SizeFilter{
		less_than:    args[0].(float64),
		greater_than: args[1].(float64),
		equal_to:     args[2].(float64),
	}
}
