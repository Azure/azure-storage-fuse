package filter

import (
	"context"
	"errors"
	"strings"
	"sync"
	"unicode"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// declaring all filters here
const size = "size"
const format = "format"
const regex = "regex"
const modtime = "modtime"
const tier = "tier"
const tag = "tag"

// struct used for storing files with bool (passed or !passed) in output channel
type opdata struct {
	filels   *internal.ObjAttr
	ispassed bool
}

// Interface having child as different type of filters like size, format, regex etc
type Filter interface {
	Apply(fileInfo *internal.ObjAttr) bool //Apply function defined for each filter, it takes file as input and returns wheather it passes that filter or not
}

// used for converting string given by user to ideal string so that it becomes easy to process
func StringConv(r rune) rune {
	if unicode.IsSpace(r) {
		return -1 // Remove space
	}
	if r >= 'A' && r <= 'Z' {
		return unicode.ToLower(r) // Convert uppercase to lowercase
	}
	return r
}

// used to return the name of filter
func getFilterName(str *string) string {
	for i := range *str {
		if !(((*str)[i] >= 'a' && (*str)[i] <= 'z') || ((*str)[i] >= 'A' && (*str)[i] <= 'Z')) { //assuming filters would have only alphabetic names, break when current char is not an alphabet
			return (*str)[0:i] //then return the substring till prev index , it will be the name of filter
		}
	}
	return "error" //if no substring is returned inside loop this means the filter name was not valid or does not exists
}

// it will store the fliters, outer array splitted by ||, inner array splitted by &&, tier filter presence and tag filter presence
type UserInputFilters struct {
	FilterArr [][]Filter
	TierChk   bool
	TagChk    bool
}

// this function parses the input string and stores filters, tierchk and tagchk in UserInputFilters
func (fl *UserInputFilters) ParseInp(str *string) error {
	splitOr := strings.Split((*str), "||") //splitted string on basis of OR
	fl.TierChk = false
	fl.TagChk = false
	for _, andFilters := range splitOr { //going over each part splitted by OR
		var individualFilter []Filter               //this array will store all filters seperated by && at each index
		splitAnd := strings.Split(andFilters, "&&") //splitted by &&
		for _, singleFilter := range splitAnd {     //this gives a particular filter (ex- A&&B&&C so it will traverse A then B then C)
			trimmedStr := strings.TrimSpace(singleFilter)
			thisFilter := getFilterName(&trimmedStr) //retrieve name of filter
			thisFilter = strings.ToLower(thisFilter) //converted to lowercase
			var obj Filter
			var erro error
			if thisFilter == size {
				obj, erro = giveSizeFilterObj(&singleFilter)
			} else if thisFilter == format {
				obj, erro = giveFormatFilterObj(&singleFilter)
			} else if thisFilter == regex {
				obj, erro = giveRegexFilterObj(&singleFilter)
			} else if thisFilter == modtime {
				obj, erro = giveModtimeFilterObj(&singleFilter)
			} else if thisFilter == tier {
				if !fl.TierChk { //if tier filter is present , set tierchk to true
					fl.TierChk = true
				}
				obj, erro = giveAccessTierFilterObj(&singleFilter)
			} else if thisFilter == tag {
				if !fl.TagChk { //if tag filter is present , set tagchk to true
					fl.TagChk = true
				}
				obj, erro = giveBlobTagFilterObj(&singleFilter)
			} else { // if no name matched , means it is not a valid filter , thus return an error
				return errors.New("invalid filter, no files passed")
			}
			if erro != nil { //if any filter provided error while parsing return error
				return erro
			}
			individualFilter = append(individualFilter, obj) //inner array (splitted by &&) is being formed
		}
		fl.FilterArr = append(fl.FilterArr, individualFilter) //outer array (splitted by ||) is being formed
	}
	return nil //everything went well, no error
}

type FileValidator struct {
	workers      int                    //no of threads analysing file
	fileCnt      int64                  //total number of files that will be checked against filters
	wgo          sync.WaitGroup         //to wait until all files from output channel are processed
	fileInpQueue chan *internal.ObjAttr //file input channel
	outputChan   chan *opdata           //file output channel (containing both passed and !passed files)
	FilterArr    [][]Filter             //stores filters
	finalFiles   []*internal.ObjAttr    //list containing files files which passed filters
}

// read output channel
func (fv *FileValidator) RecieveOutput() {
	defer fv.wgo.Done()
	var counter int64 = 0 //keeps count of number of files recieved in output channel
	for data := range fv.outputChan {
		counter++
		// fmt.Println("OutPut Channel: ", data.filels.Name, " ", data.ispassed)  DEBUG PRINT
		if data.ispassed { //if files passed filter , append it to list of final files
			fv.finalFiles = append(fv.finalFiles, data.filels)
		}
		if counter == fv.fileCnt { //indicates that all files are processed and read from output channel , close channel now
			close(fv.outputChan)
			break
		}
	}
}

// it checks every single file against all and filters in seq order
func (fv *FileValidator) checkIndividual(ctx *context.Context, fileInf *internal.ObjAttr, filters *[]Filter) bool {
	for _, filter := range *filters {
		select {
		case <-(*ctx).Done(): // If any one combination returns true, no need to check furthur
			// fmt.Println("terminating file by context: ", (*fileInf).Name, " for filter: ", filter)  DEBUG PRINT
			return false
		default:
			passedThisFilter := filter.Apply(fileInf)
			if !passedThisFilter { //if any filter fails, return false immediately as it can never be true
				// fmt.Println("terminating file by false : ", (*fileInf).Name, " for filter: ", filter)  DEBUG PRINT
				return false
			}
		}
	}
	// fmt.Println("chkIn : ", (*fileInf))
	return true // if all filters in seq order passes , return true
}

// it takes a single file and all filters mentioned by user returns a bool
func (fv *FileValidator) CheckFileWithFilters(fileInf *internal.ObjAttr) bool {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	response := false
	resultChan := make(chan bool, len(fv.FilterArr)) // made a channel to store result calculated by each combination
	for _, filters := range fv.FilterArr {
		go func(filters []Filter) { //all combinations are running parallely to speed up
			passed := fv.checkIndividual(&ctx, fileInf, &filters)
			resultChan <- passed //push result of each combination in channel
		}(filters)
	}
	for range fv.FilterArr {
		resp := <-resultChan //here we check the result of each combination as upper for loop pushed in channel
		if (resp) && (!response) {
			cancel()
			// for the first time when we recieve a true , we will cancel context and wait for all processes to stop
		}
		response = (response || resp)
	}
	// fmt.Println("chkfil: ", (*fileInf), " ", response)
	return response // return response, it will be true if any combination returns a true
}

// this is thread pool , where 16 threads are running
func (fv *FileValidator) ChkFile() {
	for fileInf := range fv.fileInpQueue {
		if fileInf.IsDir() { //pass all directories as it is without applying filter on them
			fv.outputChan <- (&opdata{filels: fileInf, ispassed: true})
			continue
		}
		// fmt.Println("sending for check: ", fileInf.Name)
		Passed := fv.CheckFileWithFilters(fileInf)
		if Passed { //if a file passes add it to output channel with true
			fv.outputChan <- (&opdata{filels: fileInf, ispassed: true})
		} else { //if a file passes add it to output channel with false
			fv.outputChan <- (&opdata{filels: fileInf, ispassed: false})
		}
	}
}
