package filter

import (
	"errors"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

const lenTag = len(tag)

type BlobTagFilter struct {
	key   string
	value string
}

func (filter BlobTagFilter) Apply(fileInfo *internal.ObjAttr) bool {
	// fmt.Println("BlobTag filter ", filter, " file name ", (*fileInfo).Name)  DEBUG PRINT
	if val, ok := fileInfo.Tags[filter.key]; ok {
		return (filter.value == strings.ToLower(val))
	}
	return false
}

// used for dynamic creation of BlobTagFilter
func newBlobTagFilter(args ...interface{}) Filter {
	return BlobTagFilter{
		key:   args[0].(string),
		value: args[1].(string),
	}
}

func giveBlobTagFilterObj(singleFilter *string) (Filter, error) {
	(*singleFilter) = strings.Map(StringConv, (*singleFilter)) //remove all spaces and make all upperCase to lowerCase
	sinChk := (*singleFilter)[lenTag : lenTag+1]               //single char after tag (ex- tag=hot:yes , here sinChk will be "=")
	erro := errors.New("invalid blobtag filter, no files passed")
	if !(sinChk == "=") {
		return nil, erro
	}
	splitEq := strings.Split(*singleFilter, "=")
	if len(splitEq) == 2 {
		splitCol := strings.Split(splitEq[1], ":")
		if len(splitCol) == 2 {
			tagKey := splitCol[0]
			tagVal := splitCol[1]
			return newBlobTagFilter(tagKey, tagVal), nil
		} else {
			return nil, erro
		}
	} else {
		return nil, erro
	}
}
