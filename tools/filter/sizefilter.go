package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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

func giveSizeFilterObj(singleFilter string, thisFilter string, filterMap map[string]filterCreator) (Filter, bool) {
	singleFilter = strings.Map(StringConv, singleFilter)
	value := singleFilter[len(thisFilter)+1:]
	floatVal, err := strconv.ParseFloat(value, 64)
	if err != nil {
		if singleFilter[len(thisFilter)+1] != '=' {
			return nil, false
		} else {
			value := singleFilter[len(thisFilter)+2:]
			floatVal, err = strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, false
			}
			return filterMap[thisFilter](singleFilter[len(thisFilter):len(thisFilter)+2], floatVal), true
		}
	} else {
		return filterMap[thisFilter](singleFilter[len(thisFilter):len(thisFilter)+1], floatVal), true
	}
}
