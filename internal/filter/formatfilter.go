package filter

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

const lenformat = len(format)

// formatFilter and its attributes
type FormatFilter struct {
	ext_type string
}

// Apply fucntion for format filter , check wheather a file passes the constraints
func (filter FormatFilter) Apply(fileInfo *internal.ObjAttr) bool {
	// fmt.Println("Format Filter ", filter, " file name ", (*fileInfo).Name)  DEBUG PRINT
	fileExt := filepath.Ext((*fileInfo).Name)
	chkstr := "." + filter.ext_type
	// fmt.Println(fileExt, " For file :", fileInfo.Name)
	return chkstr == fileExt
}

// used for dynamic creation of formatFilter using map
func newFormatFilter(args ...interface{}) Filter {
	return FormatFilter{
		ext_type: args[0].(string),
	}
}

func giveFormatFilterObj(singleFilter *string) (Filter, error) {
	(*singleFilter) = strings.Map(StringConv, (*singleFilter))
	erro := errors.New("invalid format filter, no files passed")
	if (len((*singleFilter)) <= lenformat+1) || ((*singleFilter)[lenformat] != '=') || (!((*singleFilter)[lenformat+1] >= 'a' && (*singleFilter)[lenformat+1] <= 'z')) { //since len(format) = 6, at next position (ie index 6) there should be "=" only and assuming extention type starts from an alphabet
		return nil, erro
	}
	value := (*singleFilter)[lenformat+1:]
	return newFormatFilter(value), nil
}
