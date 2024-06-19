package filter

import (
	"errors"
	"regexp"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

const lenregex = len(regex)

// RegexFilter and its attributes
type regexFilter struct {
	regex_inp *regexp.Regexp
}

// Apply fucntion for regex filter , check wheather a file passes the constraints
func (filter regexFilter) Apply(fileInfo *internal.ObjAttr) bool {
	// fmt.Println("regex filter ", filter.regex_inp, " file name ", (*fileInfo).Name)  DEBUG PRINT
	return filter.regex_inp.MatchString((*fileInfo).Name)
}

// used for dynamic creation of regexFilter
func newRegexFilter(args ...interface{}) Filter {
	return regexFilter{
		regex_inp: args[0].(*regexp.Regexp),
	}
}

func giveRegexFilterObj(singleFilter *string) (Filter, error) {
	(*singleFilter) = strings.Map(StringConv, (*singleFilter))
	erro := errors.New("invalid regex filter, no files passed")
	if (len((*singleFilter)) <= lenregex+1) || ((*singleFilter)[lenregex] != '=') { //since len(regex) = 5, at next position (ie index 5) there should be "=" pnly
		return nil, erro
	}
	value := (*singleFilter)[lenregex+1:] //len(regex)+1 = 5 + 1
	pattern, err := regexp.Compile(value)
	if err != nil {
		return nil, erro
	}
	return newRegexFilter(pattern), nil
}
