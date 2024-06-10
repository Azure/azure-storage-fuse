package filter

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

type opdata struct {
	filels   *internal.ObjAttr
	ispassed bool
}
type Filter interface { //Interface having child as different type of filters like size, format, regex etc
	Apply(fileInfo *internal.ObjAttr) bool //Apply function defined for each filter, it takes file as input and returns wheather it passes all filters or not
}

// type filterCreator func(...interface{}) Filter //used to create object of different filter using map

func StringConv(r rune) rune { //used for converting string given by user to ideal string so that it becomes easy to process
	if unicode.IsSpace(r) {
		return -1 // Remove space
	}
	if r >= 'A' && r <= 'Z' {
		return unicode.ToLower(r) // Convert uppercase to lowercase
	}
	return r
}

func getFilterName(str *string) string { //used to return the name of filter
	for i := range *str {
		if !(((*str)[i] >= 'a' && (*str)[i] <= 'z') || ((*str)[i] >= 'A' && (*str)[i] <= 'Z')) { //assuming filters would have only alphabetic names, break when current char is not an alphabet
			return (*str)[0:i] //then return the substring till prev index , it will be the name of filter
		}
	}
	return "error" //if no substring is returned inside loop this means there was an error in input provided
}

func ParseInp(str *string) ([][]Filter, bool) { //this function parses the input string and returns an array of array of filters
	splitOr := strings.Split((*str), "||") //splitted string on basis of OR
	var filterArr [][]Filter

	// filterMap := map[string]filterCreator{ //Created a Map that will be used to create new filter objects
	// 	"size":    newSizeFilter,
	// 	"format":  newFormatFilter,
	// 	"regex":   newRegexFilter,
	// 	"modtime": newModTimeFilter, //Pushing every filter in the map, key is the name of filter while value is a dynamic constructor of filter
	// }

	for _, andFilters := range splitOr {
		var individualFilter []Filter //this array will store all filters seperated by && at each index
		splitAnd := strings.Split(andFilters, "&&")
		for _, singleFilter := range splitAnd {
			trimmedStr := strings.TrimSpace(singleFilter)
			thisFilter := getFilterName(&trimmedStr) //retrieve name of filter
			thisFilter = strings.ToLower(thisFilter) //converted to lowercase
			// TODO::filter: error checks for invalid input like size1234, size>=, format pdf
			var obj Filter
			var isvalid bool
			if thisFilter == "size" {
				obj, isvalid = giveSizeFilterObj(&singleFilter)
			} else if thisFilter == "format" {
				obj, isvalid = giveFormatFilterObj(&singleFilter)
			} else if thisFilter == "regex" {
				obj, isvalid = giveRegexFilterObj(&singleFilter)
			} else if thisFilter == "modtime" {
				obj, isvalid = giveModtimeFilterObj(&singleFilter)
			} else { // if no name matched , means it is not a valid filter , thus return a false
				return filterArr, false
			}
			if !isvalid {
				return filterArr, false
			}
			individualFilter = append(individualFilter, obj)
		}
		filterArr = append(filterArr, individualFilter)
	}
	return filterArr, true
}

type fileValidator struct {
	workers    int
	atomicflag int32 //TO DO chk bool
	fileCnt    int64
	wgo        sync.WaitGroup
	// wgi          sync.WaitGroup
	fileInpQueue chan *internal.ObjAttr
	outputChan   chan *opdata
	filterArr    [][]Filter
	finalFiles   []*internal.ObjAttr
}

func (fv *fileValidator) RecieveOutput() {
	defer fv.wgo.Done()
	var counter int64 = 0
	for data := range fv.outputChan {
		counter++
		fmt.Println("OutPut Channel: ", data.filels.Name, " ", data.ispassed)
		if data.ispassed {
			fmt.Println("In finalFiles : ", data.filels.Name)
			fv.finalFiles = append(fv.finalFiles, data.filels)
		}
		// Check if the atomic variable is true
		if (atomic.LoadInt32(&fv.atomicflag) == 1) && (counter == fv.fileCnt) {
			close(fv.outputChan)
			break
		}
	}
}
func (fv *fileValidator) checkIndividual(ctx *context.Context, fileInf *internal.ObjAttr, filters *[]Filter) bool { //it checks every single file against all and filters (as stored in 1 index of filterArr) in seq order
	for _, filter := range *filters {
		select {
		case <-(*ctx).Done(): // If any one combination returns true, no need to check furthur
			fmt.Println("terminating file by context: ", (*fileInf).Name, " for filter: ", filter)
			return false
		default:
			passedThisFilter := filter.Apply(fileInf)
			if !passedThisFilter { //if any filter fails, return false immediately as it can never be true
				fmt.Println("terminating file by false : ", (*fileInf).Name, " for filter: ", filter)
				return false
			}
		}
	}
	fmt.Println("chkIn : ", (*fileInf))
	return true // if all filters in seq order passes , return true
}

func (fv *fileValidator) checkFileWithFilters(fileInf *internal.ObjAttr) bool { // it takes a single file and all filters mentioned by user returns a bool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	response := false
	resultChan := make(chan bool, len(fv.filterArr)) // made a channel to store result calculated by each combination
	for _, filters := range fv.filterArr {
		go func(filters []Filter) { //all combinations are running parallely to speed up
			passed := fv.checkIndividual(&ctx, fileInf, &filters)
			resultChan <- passed //push result of each combination in channel
		}(filters)
	}
	for range fv.filterArr {
		resp := <-resultChan //here we check the result of each combination as upper for loop pushed in channel
		fmt.Println("banda recieved : ", fileInf.Name, " ", resp)
		if (resp) && (!response) {
			cancel()
			// for the first time when we recieve a true , we will cancel context and wait for all processes to stop
		}
		response = (response || resp)
	}
	fmt.Println("chkfil: ", (*fileInf), " ", response)
	return response // return response, it will be true if any combination returns a true
}

func (fv *fileValidator) ChkFile() { // this is thread pool , where 16 tgreads are running
	// defer fv.wgi.Done()
	for fileInf := range fv.fileInpQueue {
		// fmt.Println("worker verifing file ", fileInf.Name)
		fmt.Println("sending for check: ", fileInf.Name)
		Passed := fv.checkFileWithFilters(fileInf)
		if Passed { //if a file passes add it to result
			fmt.Println("Final Output: ", fileInf.Name)
			fv.outputChan <- (&opdata{filels: fileInf, ispassed: true})
		} else {
			fmt.Println("Not Output: ", fileInf.Name, " passing ", Passed)
			fv.outputChan <- (&opdata{filels: fileInf, ispassed: false})
		}
	}
	// fmt.Println("worker ", id, " stopped")
}
