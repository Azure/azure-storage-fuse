package filter

import (
	"errors"
	"strconv"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

const lensize = len(size)

// SizeFilter and its attributes
type SizeFilter struct {
	opr   string
	value float64
}

// Apply function for size filter , check wheather a file passes the constraints
func (filter SizeFilter) Apply(fileInfo *internal.ObjAttr) bool {
	// fmt.Println("size filter ", filter, " file name ", (*fileInfo).Name)  DEBUG PRINT

	if (filter.opr == "<=") && ((*fileInfo).Size <= int64(filter.value)) {
		return true
	} else if (filter.opr == ">=") && ((*fileInfo).Size >= int64(filter.value)) {
		return true
	} else if (filter.opr == ">") && ((*fileInfo).Size > int64(filter.value)) {
		return true
	} else if (filter.opr == "<") && ((*fileInfo).Size < int64(filter.value)) {
		return true
	} else if (filter.opr == "=") && ((*fileInfo).Size == int64(filter.value)) {
		return true
	}
	return false
}

// used for dynamic creation of sizeFilter
func newSizeFilter(args ...interface{}) Filter {
	return SizeFilter{
		opr:   args[0].(string),
		value: args[1].(float64),
	}
}

func giveSizeFilterObj(singleFilter *string) (Filter, error) {
	(*singleFilter) = strings.Map(StringConv, (*singleFilter)) //remove all spaces and make all upperCase to lowerCase
	sinChk := (*singleFilter)[lensize : lensize+1]             //single char after size (ex- size=7888 , here sinChk will be "=")
	doubChk := (*singleFilter)[lensize : lensize+2]            //2 chars after size (ex- size>=8908 , here doubChk will be ">=")
	erro := errors.New("invalid size filter, no files passed")
	if !((sinChk == "=") || (sinChk == ">") || (sinChk == "<") || (doubChk == ">=") || (doubChk == "<=")) {
		return nil, erro
	}
	value := (*singleFilter)[lensize+1:] // len(size)+1 = 4 and + 1
	floatVal, err := strconv.ParseFloat(value, 64)
	if err != nil {
		if (*singleFilter)[lensize+1] != '=' {
			return nil, erro
		} else {
			value := (*singleFilter)[lensize+2:] // len(size)+2 = 4 and + 2
			floatVal, err = strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, erro
			}
			return newSizeFilter((*singleFilter)[lensize:lensize+2], floatVal), nil // it will give operator ex "<="
		}
	} else {
		return newSizeFilter((*singleFilter)[lensize:lensize+1], floatVal), nil
	}
}
