package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

type Filter interface { //Interface having child as different type of filters like size, format, regex etc
	Apply(fileInfo os.FileInfo) bool //Apply function defined for each filter, it takes file as input and returns wheather it passes all filters or not
}

type filterCreator func(...interface{}) Filter //used to create object of different filter using map

func StringConv(r rune) rune { //used for converting string given by user to ideal string so that it becomes easy to process
	if unicode.IsSpace(r) {
		return -1 // Remove space
	}
	if r >= 'A' && r <= 'Z' {
		return unicode.ToLower(r) // Convert uppercase to lowercase
	}
	return r
}

func getFilterName(str string) string { //used to return the name of filter
	for i := range str {
		if !((str[i] >= 'a' && str[i] <= 'z') || (str[i] >= 'A' && str[i] <= 'Z')) { //assuming filters would have only alphabetic names, break when current char is not an alphabet
			return str[0:i] //then return the substring till prev index , it will be the name of filter
		}
	}
	return "error" //if no substring is returned inside loop this means there was an error in input provided
}

func ParseInp(str string) ([][]Filter, bool) { //this function parses the input string and returns an array of array of filters
	splitOr := strings.Split(str, "||") //splitted string on basis of OR
	var filterArr [][]Filter

	filterMap := map[string]filterCreator{ //Created a Map that will be used to create new filter objects
		"size":    newSizeFilter,
		"format":  newFormatFilter, //Pushing every filter in the map, key is the name of filter while value is a dynamic constructor of filter
		"regex":   newRegexFilter,
		"modtime": newModTimeFilter,
	}

	for _, andFilters := range splitOr {
		var individualFilter []Filter //this array will store all filters seperated by && at each index
		splitAnd := strings.Split(andFilters, "&&")
		for _, singleFilter := range splitAnd {
			trimmedStr := strings.TrimSpace(singleFilter)
			thisFilter := getFilterName(trimmedStr) //retrieve name of filter
			// TODO::filter: error checks for invalid input like size1234, size>=, format pdf
			if strings.ToLower(thisFilter) == "size" {
				singleFilter = strings.Map(StringConv, singleFilter)
				value := singleFilter[len(thisFilter)+1:]
				floatVal, err := strconv.ParseFloat(value, 64)
				if err != nil {
					if singleFilter[len(thisFilter)+1] != '=' {
						return filterArr, false
					} else {
						value := singleFilter[len(thisFilter)+2:]
						floatVal, err = strconv.ParseFloat(value, 64)
						if err != nil {
							return filterArr, false
						}
						individualFilter = append(individualFilter, filterMap[thisFilter](singleFilter[len(thisFilter):len(thisFilter)+2], floatVal))
					}
				} else {
					individualFilter = append(individualFilter, filterMap[thisFilter](singleFilter[len(thisFilter):len(thisFilter)+1], floatVal))
				}
				// if singleFilter[len(thisFilter):len(thisFilter)+2] == "<=" {
				// 	individualFilter = append(individualFilter, filterMap[thisFilter](floatVal, -1.0, floatVal))
				// } else if singleFilter[len(thisFilter):len(thisFilter)+2] == ">=" {
				// 	individualFilter = append(individualFilter, filterMap[thisFilter](-1.0, floatVal, floatVal))
				// } else if singleFilter[len(thisFilter)] == '>' {
				// 	individualFilter = append(individualFilter, filterMap[thisFilter](-1.0, floatVal, -1.0))
				// } else if singleFilter[len(thisFilter)] == '<' {
				// 	individualFilter = append(individualFilter, filterMap[thisFilter](floatVal, -1.0, -1.0))
				// } else if singleFilter[len(thisFilter)] == '=' { // TODO::filter: check ==
				// 	individualFilter = append(individualFilter, filterMap[thisFilter](-1.0, -1.0, floatVal))
				// } else {
				// 	return filterArr, false
				// }
			} else if strings.ToLower(thisFilter) == "format" {
				singleFilter = strings.Map(StringConv, singleFilter)
				if (len(singleFilter) <= len(thisFilter)+1) || (singleFilter[len(thisFilter)] != '=') || (!(singleFilter[len(thisFilter)+1] >= 'a' && singleFilter[len(thisFilter)+1] <= 'z')) {
					return filterArr, false
				}
				value := singleFilter[len(thisFilter)+1:]
				individualFilter = append(individualFilter, filterMap[thisFilter](value))
			} else if strings.ToLower(thisFilter) == "regex" {
				singleFilter = strings.Map(StringConv, singleFilter)
				if (len(singleFilter) <= len(thisFilter)+1) || (singleFilter[len(thisFilter)] != '=') {
					return filterArr, false
				}
				value := singleFilter[len(thisFilter)+1:]
				pattern, err := regexp.Compile(value)
				if err != nil {
					return filterArr, false
				}
				individualFilter = append(individualFilter, filterMap[thisFilter](pattern))
			} else if strings.ToLower(thisFilter) == "modtime" {
				if strings.Contains(singleFilter, "<=") {
					splitedParts := strings.Split(singleFilter, "<=")
					timeRFC1123str := strings.TrimSpace(splitedParts[1])
					timeRFC1123, _ := time.Parse(time.RFC1123, timeRFC1123str)
					individualFilter = append(individualFilter, filterMap[thisFilter]("<=", timeRFC1123))
				} else if strings.Contains(singleFilter, ">=") {
					splitedParts := strings.Split(singleFilter, ">=")
					timeRFC1123str := strings.TrimSpace(splitedParts[1])
					timeRFC1123, _ := time.Parse(time.RFC1123, timeRFC1123str)
					individualFilter = append(individualFilter, filterMap[thisFilter](">=", timeRFC1123))
				} else if strings.Contains(singleFilter, "<") {
					splitedParts := strings.Split(singleFilter, "<")
					timeRFC1123str := strings.TrimSpace(splitedParts[1])
					timeRFC1123, _ := time.Parse(time.RFC1123, timeRFC1123str)
					individualFilter = append(individualFilter, filterMap[thisFilter]("<", timeRFC1123))
				} else if strings.Contains(singleFilter, ">") {
					splitedParts := strings.Split(singleFilter, ">")
					timeRFC1123str := strings.TrimSpace(splitedParts[1])
					timeRFC1123, _ := time.Parse(time.RFC1123, timeRFC1123str)
					individualFilter = append(individualFilter, filterMap[thisFilter](">", timeRFC1123))
				} else if strings.Contains(singleFilter, "=") {
					splitedParts := strings.Split(singleFilter, "=")
					timeRFC1123str := strings.TrimSpace(splitedParts[1])
					timeRFC1123, _ := time.Parse(time.RFC1123, timeRFC1123str)
					individualFilter = append(individualFilter, filterMap[thisFilter]("=", timeRFC1123))
				} else {
					return filterArr, false
				}
				// value := singleFilter[len(thisFilter)+1:]
				// utcTime, err := ConvertRFC1123(value)
				// if err != nil {
				// 	if singleFilter[len(thisFilter)+1] != '=' {
				// 		// fmt.Println("isse")
				// 		return filterArr, false
				// 	}
				// 	value := singleFilter[len(thisFilter)+2:]
				// 	fmt.Println(value)
				// 	utcTime, err = ConvertRFC1123(value)
				// 	if err != nil {
				// 		fmt.Println("isse")
				// 		return filterArr, false
				// 	}
				// }
				// fmt.Println(utcTime)
				// var zeroTime time.Time
				// if singleFilter[len(thisFilter):len(thisFilter)+2] == "<=" {
				// 	individualFilter = append(individualFilter, filterMap[thisFilter](utcTime, zeroTime, utcTime))
				// } else if singleFilter[len(thisFilter):len(thisFilter)+2] == ">=" {
				// 	individualFilter = append(individualFilter, filterMap[thisFilter](zeroTime, utcTime, utcTime))
				// } else if singleFilter[len(thisFilter)] == '>' {
				// 	individualFilter = append(individualFilter, filterMap[thisFilter](zeroTime, utcTime, zeroTime))
				// } else if singleFilter[len(thisFilter)] == '<' {
				// 	individualFilter = append(individualFilter, filterMap[thisFilter](utcTime, zeroTime, zeroTime))
				// } else if singleFilter[len(thisFilter)] == '=' { // TODO::filter: check ==
				// 	individualFilter = append(individualFilter, filterMap[thisFilter](zeroTime, zeroTime, utcTime))
				// } else {
				// 	fmt.Println("isse")
				// 	return filterArr, false
				// }
			} else { // if no name matched , means it is not a valid filter , thus return a false
				return filterArr, false
			}
		}
		filterArr = append(filterArr, individualFilter)
	}
	return filterArr, true
}
func checkIndividual(ctx context.Context, fileInf os.FileInfo, filters []Filter) bool { //it checks every single file against all and filters (as stored in 1 index of filterArr) in seq order
	for _, filter := range filters {
		select {
		case <-ctx.Done(): // If any one combination returns true, no need to check furthur
			fmt.Println("terminating file by context: ", fileInf.Name(), " for filter: ", filter)
			return true
		default:
			passedThisFilter := filter.Apply(fileInf)
			if !passedThisFilter { //if any filter fails, return false immediately as it can never be true
				fmt.Println("terminating file by false : ", fileInf.Name(), " for filter: ", filter)
				return false
			}
		}
	}
	return true // if all filters in seq order passes , return true
}

func checkFileWithFilters(fileInf os.FileInfo, filterArr [][]Filter) bool { // it takes a single file and all filters mentioned by user returns a bool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	response := false
	resultChan := make(chan bool, len(filterArr)) // made a channel to store result calculated by each combination
	for _, filters := range filterArr {
		go func(filters []Filter) { //all combinations are running parallely to speed up
			passed := checkIndividual(ctx, fileInf, filters)
			resultChan <- passed //push result of each combination in channel
		}(filters)
	}
	for range filterArr {
		resp := <-resultChan //here we check the result of each combination as upper for loop pushed in channel
		if (resp) && (!response) {
			cancel()
			// return true //if any response is true , we will stop and return true, defer cancel() will also run and thus ctx.Done() is also done
		}
		response = (response || resp)
	}
	return response //if no combination returns a true, we will return false, that is exclude this file
}

func ChkFile(id int, fileInpQueue <-chan os.FileInfo, wg *sync.WaitGroup, filterArr [][]Filter) { // this is thread pool , where 16 tgreads are running
	defer wg.Done()
	for fileInf := range fileInpQueue {
		Passed := checkFileWithFilters(fileInf, filterArr)
		if Passed { //if a file passes add it to result
			fmt.Println("Final Output: ", fileInf.Name())
		}
		// fmt.Println("worker ", id, " verifing file ", fileInf.Name())
	}
	fmt.Println("worker ", id, " stopped")
}
