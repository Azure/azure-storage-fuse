package main

import (
	"fmt"
	"os"
	"regexp"
)

type regexFilter struct { //RegexFilter and its attributes
	regex_inp *regexp.Regexp
}

func (filter regexFilter) Apply(fileInfo os.FileInfo) bool { //Apply fucntion for regex filter , check wheather a file passes the constraints
	fmt.Println("regex filter ", filter, " file name ", fileInfo.Name())
	// baseName := strings.TrimSuffix(fileInfo.Name(), filepath.Ext(fileInfo.Name()))
	// fmt.Println(baseName, "yeh rha")
	// pattern, err := regexp.Compile(filter.regex_inp) //TO DO: only once
	// if err != nil {
	// 	fmt.Println("Invalid regex pattern:", err)
	// 	return false
	// }
	return filter.regex_inp.MatchString(fileInfo.Name())
	// if pattern.MatchString(fileInfo.Name()) {
	// 	return true
	// }
	// return false
}

func newRegexFilter(args ...interface{}) Filter { // used for dynamic creation of regexFilter using map
	return regexFilter{
		regex_inp: args[0].(*regexp.Regexp),
	}
}
