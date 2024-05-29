package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Filter interface {
	Apply() bool
}
type SizeFilter struct {
	less_than    float64
	greater_than float64
	equal_to     float64
}
type FormatFilter struct {
	ext_type string
}

func (fl SizeFilter) Apply() bool {
	fmt.Println("size filter called")
	fmt.Println("At this point data is ", fl)
	return true
}
func (fl FormatFilter) Apply() bool {
	fmt.Println("FormatFilter called")
	fmt.Println("At this point data is ", fl)
	return true
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
					return filterArr, false
				}
				if SingleFilter[len(thisFilter)] == '>' {
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
}
