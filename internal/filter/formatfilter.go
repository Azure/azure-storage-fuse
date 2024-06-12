package filter

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

type FormatFilter struct { //formatFilter and its attributes
	ext_type string
}

func (filter FormatFilter) Apply(fileInfo *internal.ObjAttr) bool { //Apply fucntion for format filter , check wheather a file passes the constraints
	fmt.Println("Format Filter ", filter, " file name ", (*fileInfo).Name)
	fileExt := filepath.Ext((*fileInfo).Name)
	chkstr := "." + filter.ext_type
	// fmt.Println(fileExt, " For file :", fileInfo.Name)
	return chkstr == fileExt
}

func newFormatFilter(args ...interface{}) Filter { // used for dynamic creation of formatFilter using map
	return FormatFilter{
		ext_type: args[0].(string),
	}
}

func giveFormatFilterObj(singleFilter *string) (Filter, error) {
	(*singleFilter) = strings.Map(StringConv, (*singleFilter))
	erro := errors.New("invalid filter, no files passed")
	if (len((*singleFilter)) <= 7) || ((*singleFilter)[6] != '=') || (!((*singleFilter)[7] >= 'a' && (*singleFilter)[7] <= 'z')) {
		return nil, erro
	}
	value := (*singleFilter)[7:] //7 is used because len(format) = 6 + 1
	return newFormatFilter(value), nil
}
