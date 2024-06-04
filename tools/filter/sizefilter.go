package main

import (
	"fmt"
	"os"
)

type SizeFilter struct { //SizeFilter and its attributes
	opr   string
	value float64
}

func (filter SizeFilter) Apply(fileInfo os.FileInfo) bool { //Apply fucntion for size filter , check wheather a file passes the constraints
	fmt.Println("size filter ", filter, " file name ", fileInfo.Name())
	if (filter.opr == "<=") && (fileInfo.Size() <= int64(filter.value)) {
		return true
	} else if (filter.opr == ">=") && (fileInfo.Size() >= int64(filter.value)) {
		return true
	} else if (filter.opr == ">") && (fileInfo.Size() > int64(filter.value)) {
		return true
	} else if (filter.opr == "<") && (fileInfo.Size() < int64(filter.value)) {
		return true
	} else if (filter.opr == "=") && (fileInfo.Size() == int64(filter.value)) {
		return true
	}
	return false
}

func newSizeFilter(args ...interface{}) Filter { // used for dynamic creation of sizeFilter using map
	return SizeFilter{
		opr:   args[0].(string),
		value: args[1].(float64),
	}
}
