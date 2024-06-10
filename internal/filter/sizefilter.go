package filter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

type SizeFilter struct { //SizeFilter and its attributes
	opr   string
	value float64
}

func (filter SizeFilter) Apply(fileInfo *internal.ObjAttr) bool { //Apply fucntion for size filter , check wheather a file passes the constraints
	fmt.Println("size filter ", filter, " file name ", (*fileInfo).Name)
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

func newSizeFilter(args ...interface{}) Filter { // used for dynamic creation of sizeFilter using map
	return SizeFilter{
		opr:   args[0].(string),
		value: args[1].(float64),
	}
}

func giveSizeFilterObj(singleFilter *string) (Filter, bool) {
	(*singleFilter) = strings.Map(StringConv, (*singleFilter))
	value := (*singleFilter)[5:] // 5 is used since len(size) = 4 and + 1
	floatVal, err := strconv.ParseFloat(value, 64)
	if err != nil {
		if (*singleFilter)[5] != '=' {
			return nil, false
		} else {
			value := (*singleFilter)[6:] // 5 is used since len(size) = 4 and + 2
			floatVal, err = strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, false
			}
			return newSizeFilter((*singleFilter)[4:6], floatVal), true // 4 to 6 will give operator ex "<="
		}
	} else {
		return newSizeFilter((*singleFilter)[4:5], floatVal), true
	}
}
