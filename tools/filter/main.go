package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

type Filter interface {
	Apply(fileInfo os.FileInfo) bool
}
type SizeFilter struct {
	less_than    float64
	greater_than float64
	equal_to     float64
}
type FormatFilter struct {
	ext_type string
}

func (fl SizeFilter) Apply(fileInfo os.FileInfo) bool {
	fmt.Println("size filter called")
	fmt.Println("At this point data is ", fl)
	if (fl.less_than != -1) && (fl.equal_to != -1) && (fileInfo.Size() <= int64(fl.less_than)) {
		return true
	} else if (fl.greater_than != -1) && (fl.equal_to != -1) && (fileInfo.Size() >= int64(fl.greater_than)) {
		return true
	} else if (fl.greater_than != -1) && (fileInfo.Size() > int64(fl.greater_than)) {
		return true
	} else if (fl.less_than != -1) && (fileInfo.Size() < int64(fl.less_than)) {
		return true
	} else if (fl.equal_to != -1) && (fileInfo.Size() == int64(fl.equal_to)) {
		return true
	}
	return false
}
func (fl FormatFilter) Apply(fileInfo os.FileInfo) bool {
	fmt.Println("FormatFilter called")
	fmt.Println("At this point data is ", fl)
	fileExt := filepath.Ext(fileInfo.Name())
	chkstr := "." + fl.ext_type
	return chkstr == fileExt
}

func StringConv(r rune) rune {
	if unicode.IsSpace(r) {
		return -1 // Remove space
	}
	if r >= 'A' && r <= 'Z' {
		return unicode.ToLower(r) // Convert uppercase to lowercase
	}
	return r
}

type filterCreator func(...interface{}) Filter

func newSizeFilter(args ...interface{}) Filter {
	return SizeFilter{
		less_than:    args[0].(float64),
		greater_than: args[1].(float64),
		equal_to:     args[2].(float64),
	}
}
func newFormatFilter(args ...interface{}) Filter {
	return FormatFilter{
		ext_type: args[0].(string),
	}
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
	SplitOr := strings.Split(str, "||")
	var filterArr [][]Filter
	filterMap := map[string]filterCreator{
		"size":   newSizeFilter,
		"format": newFormatFilter,
	}
	for _, data := range SplitOr {
		var individualFilter []Filter
		SplitAnd := strings.Split(data, "&&")
		for _, SingleFilter := range SplitAnd {
			thisFilter := getFilterName(SingleFilter)
			if thisFilter == "size" {
				value := SingleFilter[len(thisFilter)+1:]
				floatVal, err := strconv.ParseFloat(value, 64)
				if err != nil {
					if SingleFilter[len(thisFilter)+1] != '=' {
						return filterArr, false
					}
					value := SingleFilter[len(thisFilter)+2:]
					floatVal, err = strconv.ParseFloat(value, 64)
					if err != nil {
						return filterArr, false
					}
				}
				if SingleFilter[len(thisFilter):len(thisFilter)+2] == "<=" {
					individualFilter = append(individualFilter, filterMap[thisFilter](floatVal, -1.0, floatVal))
				} else if SingleFilter[len(thisFilter):len(thisFilter)+2] == ">=" {
					individualFilter = append(individualFilter, filterMap[thisFilter](-1.0, floatVal, floatVal))
				} else if SingleFilter[len(thisFilter)] == '>' {
					individualFilter = append(individualFilter, filterMap[thisFilter](-1.0, floatVal, -1.0))
				} else if SingleFilter[len(thisFilter)] == '<' {
					individualFilter = append(individualFilter, filterMap[thisFilter](floatVal, -1.0, -1.0))
				} else if SingleFilter[len(thisFilter)] == '=' {
					individualFilter = append(individualFilter, filterMap[thisFilter](-1.0, -1.0, floatVal))
				}
			} else if thisFilter == "format" {
				value := SingleFilter[len(thisFilter)+1:]
				individualFilter = append(individualFilter, filterMap[thisFilter](value))
			} else {
				fmt.Println("error in input , try again ")
				return filterArr, false
			}
		}
		filterArr = append(filterArr, individualFilter)
	}
	return filterArr, true
}
func checkIndividual(ctx context.Context, FileNo os.FileInfo, filters []Filter) bool {
	for _, filter := range filters {
		select {
		case <-ctx.Done():
			return true
		default:
			passedThisFilter := filter.Apply(FileNo)
			if !passedThisFilter {
				return false
			}
		}
	}
	return true
}
func checkFileWithFilters(FileNo os.FileInfo, filterArr [][]Filter) bool {
	ctx := context.Background()
	Ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	resultChan := make(chan bool)
	for _, filters := range filterArr {
		go func(filters []Filter) {
			select {
			case <-Ctx.Done():
				return
			default:
				passed := checkIndividual(Ctx, FileNo, filters)
				if passed {
					resultChan <- passed
				}
			}
		}(filters)
	}
	select {
	case <-Ctx.Done(): //if none filter is true for a file it will wait here indefinitely
		return false
	case <-resultChan:
		return true
	}
}
func ChkFile(id int, FileInpQueue <-chan os.FileInfo, wg *sync.WaitGroup, filterArr [][]Filter) {
	defer wg.Done()
	for FileNo := range FileInpQueue {
		Passed := checkFileWithFilters(FileNo, filterArr)
		if Passed {
			fmt.Println(FileNo.Name())
		}
		fmt.Println("worker ", id, " verifing file ", FileNo.Name())
	}
	fmt.Println("worker ", id, " stopped")
}
func main() {
	filterInfo := flag.String("filterInfo", "!", "enter your filter here")
	flag.Parse()
	str := (*filterInfo)
	Modifiedstr := strings.Map(StringConv, str)
	fmt.Println(Modifiedstr)
	filterArr, isvalid := ParseInp(Modifiedstr)

	if !isvalid {
		return
	}
	for i, innerArray := range filterArr {
		fmt.Println("Inner array: ", i+1)
		for _, data := range innerArray {

			fmt.Println(data)
			// data.Apply()
		}
	}
	dirPath := "../../../TstData"
	dir, err := os.Open(dirPath)
	if err != nil {
		fmt.Println("Error opening directory:", err)
		return
	}
	fileInfos, err := dir.Readdir(-1)
	if err != nil {
		fmt.Println("error reading directory:", err)
		return
	}

	// Loop through each file in the directory
	// for _, fileInfo := range fileInfos {
	// 	// Check if the file is a regular file
	// 	if fileInfo.Mode().IsRegular() {
	// 		// Print the file name
	// 		fileExt := filepath.Ext(fileInfo.Name())
	// 		fmt.Println("File:", fileInfo.Name())
	// 		fmt.Println(fileExt)
	// 	}
	// }
	const workers = 16
	FileInpQueue := make(chan os.FileInfo, workers)
	var wg sync.WaitGroup
	for w := 1; w <= workers; w++ {
		wg.Add(1)
		go ChkFile(w, FileInpQueue, &wg, filterArr)
	}
	for _, fileinfo := range fileInfos {
		FileInpQueue <- fileinfo
	}
	close(FileInpQueue)
	wg.Wait()
	fmt.Println("All workers stopped ")
}
