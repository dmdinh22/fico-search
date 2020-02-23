package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const kb = 1024
const mb = 1024 * kb
const gb = 1024 * mb

var outputList []ResponseData

// Go doesn't have a native enum type
type StatusType string

const (
	Succ = "SUCCESS"
	Fail = "FAILURE"
	TO   = "TIMEOUT"
)

type ScannerByteCounter struct {
	BytesRead int
}

type ResponseData struct {
	Elapsed   int // in ms
	ByteCount int
	Status    string
}

type ScanFileParams struct {
	start   time.Time
	timeout time.Duration
	file    string
	offset  int64
	limit   int64
	channel chan (string)
}

func main() {
	// Set the format of the logger
	log.SetFlags(log.Ldate | log.Lmicroseconds)

	start := time.Now()
	//!TODO use 60s as def, 10s for testing for now
	timeout := 10 * 1000 * time.Millisecond
	// timeout := 60 * 1000 * time.Millisecond
	file := "./tmp/smallSample.txt"
	// fileInput := os.Stdin

	fileInfo, err := os.Stat(file)
	checkForError(err)
	fmt.Printf("the file is %d bytes long:\n", fileInfo.Size())

	writer := bufio.NewWriter(os.Stdout)
	channel := make(chan (string)) // channel used to scan the files for words in multiple goroutines
	done := make(chan (bool), 1)   // channel to signal parent that data has been entered into dict
	waitGroup := sync.WaitGroup{}  // waits for all goroutines to complete
	dict := make(map[string]int)   // stores key of elapsed time and value of response struct
	var currentBytePos int64
	var limit int64 = 1 * mb // sets limit of data chunk per goroutine

	// read all incoming words from channel and count
	go func() {
		for str := range channel {
			//TODO: make map of time and data
			// fmt.Print("str from channel", str)
			dict[str]++
		}

		fmt.Println(dict)
		// let parent know that dict has been updated
		done <- true
	}()

	// for i := 0; i < 10; i++ {
	waitGroup.Add(1)
	defer waitGroup.Wait()
	fmt.Printf("thread started \n")

	go (func() {
		params := ScanFileParams{start, timeout, file, currentBytePos, limit, channel}
		go scanFileForKeyword(params, waitGroup)

		fmt.Printf("thread completed \n")
		waitGroup.Done()
	})()
	// increment byte pos by  lastbyte read by the prev thread + 1 (account for EOL)
	currentBytePos += limit + 1
	// }

	// params := ScanFileParams{start, timeout, file, currentBytePos, limit, channel}
	// scanFileForKeyword(params, waitGroup)

	waitGroup.Wait()
	close(channel)
	writer.Flush()

	// wait for dict to process data
	<-done
	close(done)

	// sort by elapsed time desc
	sort.Slice(outputList, func(a, b int) bool {
		return outputList[a].Elapsed > outputList[b].Elapsed
	})

	// print out list
	fmt.Print(outputList)

	end := time.Now()
	log.Println("finished", end.Sub(start))
}

func checkForError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func getElapsedTime(start time.Time, name string) time.Duration {
	elapsed := time.Since(start)
	return elapsed
}

func scanFileForKeyword(input ScanFileParams, wg sync.WaitGroup) {
	// Decreasing internal counter for wait-group as soon as goroutine finishes
	defer wg.Done()

	fileToRead, err := os.Open(input.file)
	checkForError(err)
	defer fileToRead.Close()

	// move ptr to start of current chunk
	fileToRead.Seek(input.offset, 0)
	reader := bufio.NewReader(fileToRead)

	// if there is an offset, we need to shift bytes to the end of the word so the chunk starts at a new word
	if input.offset != 0 {
		_, err = reader.ReadBytes(' ')
		if err == io.EOF {
			fmt.Println("EOF")
			// EOF is a failure
			failedToMatch := ResponseData{Status: Fail}
			outputList = append(outputList, failedToMatch)
			fmt.Print(failedToMatch)
			return
		}

		if err != nil {
			// program errored out - failure
			failedToMatch := ResponseData{Status: Fail}
			outputList = append(outputList, failedToMatch)
			fmt.Print(failedToMatch)
			panic(err)
		}
	}

	var bytesReadSize int64
	for {
		// get elapsed time and check against timeout
		elapsed := getElapsedTime(input.start, "file-scan")
		if elapsed > input.timeout {
			fmt.Println("the process has timed out - elapsed:", elapsed)
			timedOutResponse := ResponseData{Status: TO}
			outputList = append(outputList, timedOutResponse)
			return
		}

		// if the size has exceeded chunk limit, break out
		if bytesReadSize > input.limit {
			break
		}

		bytesRead, err := reader.ReadBytes(' ')

		// break for end of file
		if err == io.EOF {
			break
		}

		if err != nil {
			failedToMatch := ResponseData{Status: Fail}
			outputList = append(outputList, failedToMatch)
			panic(err)
		}

		removeSpecial := regexp.MustCompile(`(?m)[^a-z]`)
		bytesReadSize += int64(len(bytesRead))
		bytesToStr := strings.TrimSpace(string(bytesRead))
		lowercasedWord := strings.ToLower(string(bytesToStr))
		sanitizedWord := strings.Replace(lowercasedWord, "\n", "", -1)
		sanitizedWord = removeSpecial.ReplaceAllString(sanitizedWord, "")
		matchedKeyword, err := regexp.MatchString("\\bfico\\b", sanitizedWord)
		checkForError(err)

		if lowercasedWord != "" {
			// Send the read word in the channel to enter into dictionary.
			input.channel <- lowercasedWord
		}

		if lowercasedWord != "" && matchedKeyword {
			elapsedForMatch := getElapsedTime(input.start, "matched")
			matchedData := ResponseData{Elapsed: int(elapsedForMatch), ByteCount: int(bytesReadSize), Status: Succ}

			fmt.Print(matchedData)
			fmt.Println("\n")
			input.channel <- matchedData.Status
			outputList = append(outputList, matchedData)
		}
	}
}

// func startWorker(lines <-chan string) <-chan string {
// 	finished := make(chan string)
// 	go func() {
// 		defer close(finished)
// 		for line := range lines {
// 			// Do your heavy work here
// 			finished <- line
// 		}
// 	}()
// 	return finished
// }

// func merge(cs ...<-chan string) <-chan string {
// 	var wg sync.WaitGroup
// 	out := make(chan string)

// 	// Start an output goroutine for each input channel in cs.  output
// 	// copies values from c to out until c is closed, then calls wg.Done.
// 	output := func(c <-chan string) {
// 		for n := range c {
// 			out <- n
// 		}
// 		wg.Done()
// 	}
// 	wg.Add(len(cs))
// 	for _, c := range cs {
// 		go output(c)
// 	}

// 	// Start a goroutine to close out once all the output goroutines are
// 	// done.  This must start after the wg.Add call.
// 	go func() {
// 		wg.Wait()
// 		close(out)
// 	}()
// 	return out
// }
