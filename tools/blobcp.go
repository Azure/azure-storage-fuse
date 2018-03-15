package main

import (
    "flag"
    "fmt"
    "io"
    "log"
    "os"
    "path/filepath"
    "sync"
    "sync/atomic"
    "time"
)

var workers = flag.Int("n", 32, "Number of workers")
var source = flag.String("s", "", "Source path to walk through")
var dest = flag.String("d", "", "Destination to copy to")
var include = flag.String("pattern", "", "Path pattern to match in source path")
var processed int64 = 0

func main() {
    flag.Parse()
    if *source == "" {
        log.Printf("-s source path is missing")
        flag.PrintDefaults()
    } else if *dest == "" {
        log.Printf("-d destination path is missing")
        flag.PrintDefaults()
    }

    pending := sync.WaitGroup{}
    files := make(chan string)

    go func() {
        last := processed
        for {
            <-time.After(5 * time.Second)
            fmt.Printf("Processed %v, %v per second       \r", processed, (processed-last)/5)
            last = processed
        }
    }()

    start := time.Now()
    pending.Add(1)
    startWorkers(files, &pending)
    submitFiles(*source, *include, files, &pending)
    pending.Wait()

    fmt.Printf("Processed %v files in %v sec", processed, time.Now().Sub(start))
}

func startWorkers(files chan string, pending *sync.WaitGroup) {
    for i := 0; i < *workers; i++ {
        go func() {
            for p := range files {
                processFile(p)
                atomic.AddInt64(&processed, 1)
                pending.Done()
            }
        }()
    }
    pending.Done()
}

func submitFiles(path string, include string, files chan<- string, pending *sync.WaitGroup) {
    var pathsFound []string
    var err error
    if include == "" {
        pathsFound = append(pathsFound, path)
    } else {
        pathsFound, err = filepath.Glob(path + "/" + include)
        if err != nil {
            log.Printf("Some error during files discovering: %v, %v", include, err)
        }
    }

    for _, path := range pathsFound {
        filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
            if err != nil {
                log.Printf("Some error during files discovering: %v, %v", p, err)
            }
            if info.Mode().IsRegular() {
                pending.Add(1)
                files <- p
            }
            return nil
        })
    }
}


func processFile(p string) {
    relPath, err := filepath.Rel(filepath.Dir(*source), p)
    destination := filepath.Join(*dest, relPath)
    f, err := os.Open(p)
    if err != nil {
        log.Print("Failed to open %v: %v", p, err)
    }
    defer f.Close()

    err = os.MkdirAll(filepath.Dir(destination), os.ModePerm)
    if err != nil {
        log.Print("Failed to create directory %v: %v", destination, err)
    }

    newf, err := os.Create(destination)
    if err != nil {
        log.Print("Failed to create %v: %v", p, err)
    }
    defer newf.Close()

    _, err = io.Copy(newf, f)
    if err != nil {
        log.Print("Failed to read from %v: %v", p, err)
    }
}
