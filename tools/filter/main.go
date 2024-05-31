package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
)

func main() {
	filter := flag.String("filter", "!", "enter your filter here")
	flag.Parse()
	str := (*filter)
	idealStr := strings.Map(StringConv, str) // TODO::filter: add comments
	fmt.Println(idealStr)
	filterArr, isInpValid := ParseInp(idealStr)

	if !isInpValid {
		// TODO::filter: log error here
		fmt.Println("Wrong input format, Try again.")
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
	const workers = 16
	fileInpQueue := make(chan os.FileInfo, workers)
	var wg sync.WaitGroup
	for w := 1; w <= workers; w++ {
		wg.Add(1)
		go ChkFile(w, fileInpQueue, &wg, filterArr)
	}
	for _, fileinfo := range fileInfos {
		fileInpQueue <- fileinfo
	}
	close(fileInpQueue)
	wg.Wait()
	fmt.Println("All workers stopped ")
}
