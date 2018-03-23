package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var workers = flag.Int("n", 32, "Number of workers")
var include = flag.String("pattern", "", "Path pattern to match in source path")

var source string
var dest string
var processed int64
var routines int64

func main() {

        // Customize usage text
        flag.Usage = func() {
            fmt.Printf("Usage: blobcp [-pattern] [-n] source destination\n\n")
            flag.PrintDefaults()
            fmt.Printf("\nEx: blobcp /home/mydir /myfiles\n")
            fmt.Printf("Ex: blobcp -pattern=\"*.txt\" -n=16 /home/mydir /myfiles\n")
        }

	// Parse command line arguments
	flag.Parse()

	// Check that given 2 arguments are source and destination
	if len(flag.Args()) < 2 {
		log.Println("Source or destination missing")
		flag.Usage()
		os.Exit(1)
	} else if len(flag.Args()) == 2 {
		for i := range flag.Args() {
			if strings.HasPrefix(flag.Args()[i], "-") {
				flag.Usage()
				os.Exit(1)
			}
		}

	}

	// Parse source and destination and print
	source = flag.Arg(0)
	dest = flag.Arg(1)

        // Print job details
        var pattern string
        if *include == "" {
           pattern = "*"
        } else {
           pattern = *include
        }
	fmt.Printf("Copying data from %v into %v that matches pattern %v\n", source, dest, pattern)

        // Synchronize through file channel
	pending := sync.WaitGroup{}
	files := make(chan string, 1024)

	// Print total, and 5 second averages
	go func() {
		last := processed
		for {
			<-time.After(5 * time.Second)
			_processed := atomic.LoadInt64(&processed)
			_routines := atomic.LoadInt64(&routines)
			fmt.Printf("Processed %v, %v per second, %v on-going                \r", _processed, (_processed-last)/5, _routines)
			last = _processed
		}
	}()

	// Start workers, walk the path and process files
	start := time.Now()
	startWorkers(files, &pending)
	submitFiles(source, *include, files, &pending)
	pending.Wait()

	fmt.Printf("Processed %v files in %v sec                 \n", processed, time.Now().Sub(start))
}

// Workers thread to consume files found in source path
func startWorkers(files chan string, pending *sync.WaitGroup) {
	for i := 0; i < *workers; i++ {
		go func() {
			for p := range files {
				processFile(p)
				atomic.AddInt64(&processed, 1)
				atomic.AddInt64(&routines, -1)
				pending.Done()
			}
		}()
	}
}

// Find the files based on the pattern provided, and submit to the 'files' channel
func submitFiles(path string, include string, files chan<- string, pending *sync.WaitGroup) {
	var pathsFound []string
	var err error
	if include == "" {
		pathsFound = append(pathsFound, path)
	} else {
		pathsFound, err = filepath.Glob(path + "/" + include)
		if err != nil {
			log.Printf("Some error during files discovering: %v, %v \n", include, err)
		}
	}

	for _, path := range pathsFound {
		filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				log.Printf("Some error during files discovering: %v, %v \n", p, err)
			}
			if info.Mode().IsRegular() {
				pending.Add(1)
				atomic.AddInt64(&routines, +1)
				files <- p
			}
			return nil
		})
	}
}

// Create the destination directory, and copy to
func processFile(p string) {
	relPath, err := filepath.Rel(filepath.Dir(source), p)
	destination := filepath.Join(dest, relPath)
	f, err := os.Open(p)
	if err != nil {
		log.Printf("Failed to open %v: %v \n", p, err)
	}
	defer f.Close()

	err = os.MkdirAll(filepath.Dir(destination), os.ModePerm)
	if err != nil {
		log.Printf("Failed to create directory %v: %v \n", destination, err)
	}

	newf, err := os.Create(destination)
	if err != nil {
		log.Printf("Failed to create %v: %v \n", p, err)
	}
	defer newf.Close()

	_, err = io.Copy(newf, f)
	if err != nil {
		log.Printf("Failed to read from %v: %v \n", p, err)
	}
}
