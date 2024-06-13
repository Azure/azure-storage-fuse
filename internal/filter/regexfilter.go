package filter

import (
	"errors"
	"regexp"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

type regexFilter struct { //RegexFilter and its attributes
	regex_inp *regexp.Regexp
}

func (filter regexFilter) Apply(fileInfo *internal.ObjAttr) bool { //Apply fucntion for regex filter , check wheather a file passes the constraints
	// fmt.Println("regex filter ", filter.regex_inp, " file name ", (*fileInfo).Name)  DEBUG PRINT
	return filter.regex_inp.MatchString((*fileInfo).Name)
}

func newRegexFilter(args ...interface{}) Filter { // used for dynamic creation of regexFilter
	return regexFilter{
		regex_inp: args[0].(*regexp.Regexp),
	}
}

func giveRegexFilterObj(singleFilter *string) (Filter, error) {
	(*singleFilter) = strings.Map(StringConv, (*singleFilter))
	erro := errors.New("invalid filter, no files passed")
	if (len((*singleFilter)) <= 6) || ((*singleFilter)[5] != '=') { //since len(regex) = 5, at next position (ie index 5) there should be "=" pnly
		return nil, erro
	}
	value := (*singleFilter)[6:] //6 is used because len(regex) = 5 + 1
	pattern, err := regexp.Compile(value)
	if err != nil {
		return nil, erro
	}
	return newRegexFilter(pattern), nil
}
