package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type modTimeFilter struct { //modTimeFilter and its attributes
	opr   string
	value time.Time
}

func (filter modTimeFilter) Apply(fileInfo os.FileInfo) bool { //Apply fucntion for modTime filter , check wheather a file passes the constraints
	fmt.Println("modTime Filter ", filter.opr, " ", filter.value, " file name ", fileInfo.Name())
	// var zerTim time.Time
	fileModTimestr := fileInfo.ModTime().UTC().Format(time.RFC1123)
	fileModTime, _ := time.Parse(time.RFC1123, fileModTimestr)
	fmt.Println(fileModTime, "this is file mod time")

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

func newModTimeFilter(args ...interface{}) Filter { // used for dynamic creation of modTimeFilter using map
	return modTimeFilter{
		opr:   args[0].(string),
		value: args[1].(time.Time),
	}
}

func giveModtimeFilterObj(singleFilter string, thisFilter string, filterMap map[string]filterCreator) (Filter, bool) {
	if strings.Contains(singleFilter, "<=") {
		splitedParts := strings.Split(singleFilter, "<=")
		timeRFC1123str := strings.TrimSpace(splitedParts[1])
		timeRFC1123, _ := time.Parse(time.RFC1123, timeRFC1123str)
		return filterMap[thisFilter]("<=", timeRFC1123), true
	} else if strings.Contains(singleFilter, ">=") {
		splitedParts := strings.Split(singleFilter, ">=")
		timeRFC1123str := strings.TrimSpace(splitedParts[1])
		timeRFC1123, _ := time.Parse(time.RFC1123, timeRFC1123str)
		return filterMap[thisFilter](">=", timeRFC1123), true
	} else if strings.Contains(singleFilter, "<") {
		splitedParts := strings.Split(singleFilter, "<")
		timeRFC1123str := strings.TrimSpace(splitedParts[1])
		timeRFC1123, _ := time.Parse(time.RFC1123, timeRFC1123str)
		return filterMap[thisFilter]("<", timeRFC1123), true
	} else if strings.Contains(singleFilter, ">") {
		splitedParts := strings.Split(singleFilter, ">")
		timeRFC1123str := strings.TrimSpace(splitedParts[1])
		timeRFC1123, _ := time.Parse(time.RFC1123, timeRFC1123str)
		return filterMap[thisFilter](">", timeRFC1123), true
	} else if strings.Contains(singleFilter, "=") {
		splitedParts := strings.Split(singleFilter, "=")
		timeRFC1123str := strings.TrimSpace(splitedParts[1])
		timeRFC1123, _ := time.Parse(time.RFC1123, timeRFC1123str)
		return filterMap[thisFilter]("=", timeRFC1123), true
	} else {
		return nil, false
	}
}

// func ConvertRFC1123(rfc1123String string) (time.Time, error) {
// 	// Split the string by comma to separate day and date-time parts
// 	parts := strings.Split(rfc1123String, ",")
// 	if len(parts) != 2 {
// 		return time.Time{}, fmt.Errorf("invalid RFC1123 string format")
// 	}

// 	// Extract the date-time part
// 	dateTimeStr := strings.TrimSpace(parts[1])

// 	// Parse the date-time string
// 	t, err := time.Parse("2Jan200615:04:05utc", dateTimeStr)
// 	if err != nil {
// 		return time.Time{}, err
// 	}

// 	// Convert the time to UTC
// 	return t.UTC(), nil
// }
