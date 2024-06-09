package filter

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type regexFilter struct { //RegexFilter and its attributes
	regex_inp *regexp.Regexp
}

func (filter regexFilter) Apply(fileInfo *os.FileInfo) bool { //Apply fucntion for regex filter , check wheather a file passes the constraints
	fmt.Println("regex filter ", filter.regex_inp, " file name ", (*fileInfo).Name())
	return filter.regex_inp.MatchString((*fileInfo).Name())
}

func newRegexFilter(args ...interface{}) Filter { // used for dynamic creation of regexFilter using map
	return regexFilter{
		regex_inp: args[0].(*regexp.Regexp),
	}
}

func giveRegexFilterObj(singleFilter *string) (Filter, bool) {
	(*singleFilter) = strings.Map(StringConv, (*singleFilter))
	if (len((*singleFilter)) <= 6) || ((*singleFilter)[5] != '=') {
		return nil, false
	}
	value := (*singleFilter)[6:] //6 is used because len(regex) = 5 + 1
	pattern, err := regexp.Compile(value)
	if err != nil {
		return nil, false
	}
	return newRegexFilter(pattern), true
}
