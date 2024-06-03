package main

import (
	"fmt"
	"os"
	"regexp"
)

type regexFilter struct { //RegexFilter and its attributes
	regex_inp string
}

func (filter regexFilter) Apply(fileInfo os.FileInfo) bool { //Apply fucntion for regex filter , check wheather a file passes the constraints
	fmt.Println("regex filter called")
	fmt.Println("At this point data is ", filter, " file name ", fileInfo.Name())
	// baseName := strings.TrimSuffix(fileInfo.Name(), filepath.Ext(fileInfo.Name()))
	// fmt.Println(baseName, "yeh rha")
	pattern, err := regexp.Compile(filter.regex_inp)
	if err != nil {
		fmt.Println("Invalid regex pattern:", err)
		return false
	}
	if pattern.MatchString(fileInfo.Name()) {
		return true
	}
	return false
}

func newRegexFilter(args ...interface{}) Filter { // used for dynamic creation of regexFilter using map
	return regexFilter{
		regex_inp: args[0].(string),
	}
}
