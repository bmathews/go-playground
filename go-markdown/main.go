package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"sync"
	"time"
)

type fileList []string

func (i *fileList) String() string {
	return "my string representation"
}

func (i *fileList) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var fileFlags fileList

func compileAsync() {
	// Create WaitGroup
	var wg sync.WaitGroup
	wg.Add(len(fileFlags))

	// Loop through files and spin off async compilations
	for _, f := range fileFlags {
		go func(path string) {
			s := time.Now()
			defer wg.Done()
			out := Compile(path)
			ioutil.WriteFile(path+".html", out, 0644)
			fmt.Printf("%s: %s -> %s\n", time.Now().Sub(s), path, path+".html")
		}(f)
	}

	// Wait for everything to complete!
	wg.Wait()
}

func compileSync() {
	for _, f := range fileFlags {
		s := time.Now()
		out := Compile(f)
		ioutil.WriteFile(f+".html", out, 0644)
		fmt.Printf("%s: %s -> %s\n", time.Now().Sub(s), f, f+".html")
	}
}

// Main
func main() {
	// Parse CLI flags
	flag.Var(&fileFlags, "files", "List of files to compile")
	flag.Parse()

	// Keep track of start time
	totalStart := time.Now()

	compileAsync()
	// compileSync()

	// Log time elapsed
	fmt.Printf("%s elapsed\n", time.Now().Sub(totalStart))
}
