package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"
	"time"
)

var (
	wg   *sync.WaitGroup
	dirs chan string = make(chan string, 50000)
)

func dirWalker(id int, ctx context.Context) error {
	defer wg.Done()
	defer fmt.Println("Ending worker ", id)

	for {
		select {
		case path := <-dirs:
			children, err := ioutil.ReadDir(path)
			if err != nil {
				fmt.Println(id, ": Failed to iterate path [%]", path)
				continue
			}

			for _, child := range children {
				childName := filepath.Join(path, child.Name())
				if child.IsDir() {
					// fmt.Println(id, ": Directory :", childName)
					select {
					case dirs <- childName:
					case <-ctx.Done():
						fmt.Println(id, ": Time up, cancelling the current operation 1...")
						return ctx.Err()
					}
				} else {
					// fmt.Println(id, ": File : ", childName)
				}
			}

		case <-ctx.Done():
			fmt.Println(id, ": Time up, cancelling the current operation 2...")
			return ctx.Err()
		}
	}
}

func main() {
	var numWorker = 10
	wg = &sync.WaitGroup{}

	ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFunc()

	// Start worker threads which wait for next directory to iterate
	for i := 1; i <= numWorker; i++ {
		wg.Add(1)
		go dirWalker(i, ctx)
	}

	dirs <- "/home/"
	wg.Wait()
	close(dirs)
}
