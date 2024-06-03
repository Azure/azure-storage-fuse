package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type modTimeFilter struct { //modTimeFilter and its attributes
	less_than    time.Time
	greater_than time.Time
	equal_to     time.Time
}

func (filter modTimeFilter) Apply(fileInfo os.FileInfo) bool { //Apply fucntion for modTime filter , check wheather a file passes the constraints
	fmt.Println("modTimeFilter called")
	fmt.Println("At this point data is ", filter.less_than, " ", filter.greater_than, " ", filter.equal_to, " file name ", fileInfo.Name())
	var zerTim time.Time
	fileModTimestr := fileInfo.ModTime().UTC().Format(time.RFC1123)
	fileModTime, _ := time.Parse(time.RFC1123, fileModTimestr)
	fmt.Println(fileModTime, "this is file mod time")

	if (filter.less_than != zerTim) && (filter.equal_to != zerTim) && (fileModTime.Before(filter.less_than) || fileModTime.Equal(filter.less_than)) {
		return true
	} else if (filter.greater_than != zerTim) && (filter.equal_to != zerTim) && (fileModTime.After(filter.greater_than) || fileModTime.Equal(filter.greater_than)) {
		fmt.Println("ja true")
		return true
	} else if (filter.greater_than != zerTim) && (fileModTime.After(filter.greater_than)) {
		return true
	} else if (filter.less_than != zerTim) && (fileModTime.Before(filter.less_than)) {
		return true
	} else if (filter.equal_to != zerTim) && (fileModTime.Equal(filter.equal_to)) {
		return true
	}
	return false
}

func newModTimeFilter(args ...interface{}) Filter { // used for dynamic creation of modTimeFilter using map
	return modTimeFilter{
		less_than:    args[0].(time.Time),
		greater_than: args[1].(time.Time),
		equal_to:     args[2].(time.Time),
	}
}

func ConvertRFC1123(rfc1123String string) (time.Time, error) {
	// Split the string by comma to separate day and date-time parts
	parts := strings.Split(rfc1123String, ",")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid RFC1123 string format")
	}

	// Extract the date-time part
	dateTimeStr := strings.TrimSpace(parts[1])

	// Parse the date-time string
	t, err := time.Parse("2Jan200615:04:05utc", dateTimeStr)
	if err != nil {
		fmt.Println("nhi bhai")
		return time.Time{}, err
	}

	// Convert the time to UTC
	return t.UTC(), nil
}
