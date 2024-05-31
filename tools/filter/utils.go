package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

type Filter interface {
	Apply(fileInfo os.FileInfo) bool
}

type filterCreator func(...interface{}) Filter

func StringConv(r rune) rune {
	if unicode.IsSpace(r) {
		return -1 // Remove space
	}
	if r >= 'A' && r <= 'Z' {
		return unicode.ToLower(r) // Convert uppercase to lowercase
	}
	return r
}

func getFilterName(str string) string {
	for i := range str {
		if !(str[i] >= 'a' && str[i] <= 'z') {
			return str[0:i]
		}
	}
	return "error"
}

func ParseInp(str string) ([][]Filter, bool) {
	splitOr := strings.Split(str, "||")
	var filterArr [][]Filter

	filterMap := map[string]filterCreator{
		"size":   newSizeFilter,
		"format": newFormatFilter,
	}

	for _, andFilters := range splitOr {
		var individualFilter []Filter
		splitAnd := strings.Split(andFilters, "&&")
		for _, singleFilter := range splitAnd {
			thisFilter := getFilterName(singleFilter)
			// TODO::filter: error checks for invalid input like size1234, size>=, format pdf
			if thisFilter == "size" {
				value := singleFilter[len(thisFilter)+1:]
				floatVal, err := strconv.ParseFloat(value, 64)
				if err != nil {
					if singleFilter[len(thisFilter)+1] != '=' {
						return filterArr, false
					}
					value := singleFilter[len(thisFilter)+2:]
					floatVal, err = strconv.ParseFloat(value, 64)
					if err != nil {
						return filterArr, false
					}
				}
				if singleFilter[len(thisFilter):len(thisFilter)+2] == "<=" {
					individualFilter = append(individualFilter, filterMap[thisFilter](floatVal, -1.0, floatVal))
				} else if singleFilter[len(thisFilter):len(thisFilter)+2] == ">=" {
					individualFilter = append(individualFilter, filterMap[thisFilter](-1.0, floatVal, floatVal))
				} else if singleFilter[len(thisFilter)] == '>' {
					individualFilter = append(individualFilter, filterMap[thisFilter](-1.0, floatVal, -1.0))
				} else if singleFilter[len(thisFilter)] == '<' {
					individualFilter = append(individualFilter, filterMap[thisFilter](floatVal, -1.0, -1.0))
				} else if singleFilter[len(thisFilter)] == '=' { // TODO::filter: check ==
					individualFilter = append(individualFilter, filterMap[thisFilter](-1.0, -1.0, floatVal))
				} else {
					return filterArr, false
				}
			} else if thisFilter == "format" {
				value := singleFilter[len(thisFilter)+1:]
				individualFilter = append(individualFilter, filterMap[thisFilter](value))
			} else {
				return filterArr, false
			}
		}
		filterArr = append(filterArr, individualFilter)
	}
	return filterArr, true
}
func checkIndividual(ctx context.Context, fileInf os.FileInfo, filters []Filter) bool {
	for _, filter := range filters {
		select {
		case <-ctx.Done():
			return true
		default:
			passedThisFilter := filter.Apply(fileInf)
			if !passedThisFilter {
				return false
			}
		}
	}
	return true
}

func checkFileWithFilters(fileInf os.FileInfo, filterArr [][]Filter) bool {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultChan := make(chan bool, len(filterArr))
	for _, filters := range filterArr {
		go func(filters []Filter) {
			passed := checkIndividual(ctx, fileInf, filters)
			resultChan <- passed
		}(filters)
	}
	for range filterArr {
		response := <-resultChan
		if response {
			return true
		}
	}
	cancel()
	return false
}

func ChkFile(id int, fileInpQueue <-chan os.FileInfo, wg *sync.WaitGroup, filterArr [][]Filter) {
	defer wg.Done()
	for fileInf := range fileInpQueue {
		Passed := checkFileWithFilters(fileInf, filterArr)
		if Passed {
			fmt.Println(fileInf.Name())
		}
		fmt.Println("worker ", id, " verifing file ", fileInf.Name())
	}
	fmt.Println("worker ", id, " stopped")
}
