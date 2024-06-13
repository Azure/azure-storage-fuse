package filter

import (
	"errors"
	"strconv"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

type SizeFilter struct { //SizeFilter and its attributes
	opr   string
	value float64
}

func (filter SizeFilter) Apply(fileInfo *internal.ObjAttr) bool { //Apply fucntion for size filter , check wheather a file passes the constraints
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

func newSizeFilter(args ...interface{}) Filter { // used for dynamic creation of sizeFilter
	return SizeFilter{
		opr:   args[0].(string),
		value: args[1].(float64),
	}
}

func giveSizeFilterObj(singleFilter *string) (Filter, error) {
	(*singleFilter) = strings.Map(StringConv, (*singleFilter)) //remove all spaces and make all upperCase to lowerCase
	sinChk := (*singleFilter)[4:5]                             //single char after size (ex- size=7888 , here sinChk will be "=")
	doubChk := (*singleFilter)[4:6]                            //2 chars after size (ex- size>=8908 , here doubChk will be ">=")
	erro := errors.New("invalid filter, no files passed")
	if !((sinChk == "=") || (sinChk == ">") || (sinChk == "<") || (doubChk == ">=") || (doubChk == "<=")) {
		return nil, erro
	}
	value := (*singleFilter)[5:] // 5 is used since len(size) = 4 and + 1
	floatVal, err := strconv.ParseFloat(value, 64)
	if err != nil {
		if (*singleFilter)[5] != '=' {
			return nil, erro
		} else {
			value := (*singleFilter)[6:] // 5 is used since len(size) = 4 and + 2
			floatVal, err = strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, erro
			}
			return newSizeFilter((*singleFilter)[4:6], floatVal), nil // 4 to 6 will give operator ex "<="
		}
	} else {
		return newSizeFilter((*singleFilter)[4:5], floatVal), nil
	}
}
