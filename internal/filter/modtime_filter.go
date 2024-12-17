package filter

import (
	"errors"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// modTimeFilter and its attributes
type modTimeFilter struct {
	opr   string
	value time.Time
}

// Apply fucntion for modTime filter , check wheather a file passes the constraints
func (filter modTimeFilter) Apply(fileInfo *internal.ObjAttr) bool {
	// fmt.Println("modTime Filter ", filter.opr, " ", filter.value, " file name ", (*fileInfo).Name)  DEBUG PRINT
	fileModTimestr := (*fileInfo).Mtime.UTC().Format(time.RFC1123)
	fileModTime, _ := time.Parse(time.RFC1123, fileModTimestr)
	// fmt.Println(fileModTime, "this is file mod time")

	if (filter.opr == "<=") && (fileModTime.Before(filter.value) || fileModTime.Equal(filter.value)) {
		return true
	} else if (filter.opr == ">=") && (fileModTime.After(filter.value) || fileModTime.Equal(filter.value)) {
		return true
	} else if (filter.opr == ">") && (fileModTime.After(filter.value)) {
		return true
	} else if (filter.opr == "<") && (fileModTime.Before(filter.value)) {
		return true
	} else if (filter.opr == "=") && (fileModTime.Equal(filter.value)) {
		return true
	}
	return false
}

// used for dynamic creation of modTimeFilter using map
func newModTimeFilter(args ...interface{}) Filter {
	return modTimeFilter{
		opr:   args[0].(string),
		value: args[1].(time.Time),
	}
}

func giveModtimeFilterObj(singleFilter *string) (Filter, error) {
	erro := errors.New("invalid Modtime filter, no files passed")
	if strings.Contains((*singleFilter), "<=") {
		splitedParts := strings.Split((*singleFilter), "<=")
		timeRFC1123str := strings.TrimSpace(splitedParts[1])
		timeRFC1123, err := time.Parse(time.RFC1123, timeRFC1123str)
		if err != nil {
			return nil, erro
		}
		return newModTimeFilter("<=", timeRFC1123), nil
	} else if strings.Contains((*singleFilter), ">=") {
		splitedParts := strings.Split((*singleFilter), ">=")
		timeRFC1123str := strings.TrimSpace(splitedParts[1])
		timeRFC1123, err := time.Parse(time.RFC1123, timeRFC1123str)
		if err != nil {
			return nil, erro
		}
		return newModTimeFilter(">=", timeRFC1123), nil
	} else if strings.Contains((*singleFilter), "<") {
		splitedParts := strings.Split((*singleFilter), "<")
		timeRFC1123str := strings.TrimSpace(splitedParts[1])
		timeRFC1123, err := time.Parse(time.RFC1123, timeRFC1123str)
		if err != nil {
			return nil, erro
		}
		return newModTimeFilter("<", timeRFC1123), nil
	} else if strings.Contains((*singleFilter), ">") {
		splitedParts := strings.Split((*singleFilter), ">")
		timeRFC1123str := strings.TrimSpace(splitedParts[1])
		timeRFC1123, err := time.Parse(time.RFC1123, timeRFC1123str)
		if err != nil {
			return nil, erro
		}
		return newModTimeFilter(">", timeRFC1123), nil
	} else if strings.Contains((*singleFilter), "=") {
		splitedParts := strings.Split((*singleFilter), "=")
		timeRFC1123str := strings.TrimSpace(splitedParts[1])
		timeRFC1123, err := time.Parse(time.RFC1123, timeRFC1123str)
		if err != nil {
			return nil, erro
		}
		return newModTimeFilter("=", timeRFC1123), nil
	} else {
		return nil, erro
	}
}
