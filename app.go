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

var outputList []ResponseData
var failedToMatch = ResponseData{Status: Fail}

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
	Elapsed     int // in ms
	ByteCount   int
	Status      string
	MatchedWord string
}

type ScanFileParams struct {
	start          time.Time
	timeout        time.Duration
	file           string
	offset         int64
	limit          int64
	matchedResults chan ResponseData
}

func main() {
	// set format of the logger
	log.SetFlags(log.Ldate | log.Lmicroseconds)

	start := time.Now()
	//!TODO use 60s as def, 10s for testing for now
	// timeout := 10 * 1000 * time.Millisecond
	timeout := 1 * time.Millisecond
	file := "./tmp/sampleInput.txt"
	// fileInput := os.Stdin

	fileInfo, err := os.Stat(file)
	checkForError(err)
	fileSize := fileInfo.Size()
	fmt.Printf("the file is %d bytes long:\n", fileInfo.Size())

	writer := bufio.NewWriter(os.Stdout)
	matchedResults := make(chan (ResponseData)) // matchedResults used to scan the files for words in multiple goroutines
	done := make(chan (bool), 1)                // matchedResults to signal parent that data has been entered into dict
	waitGroup := sync.WaitGroup{}               // waits for all goroutines to complete
	dict := make(map[string]int)                // stores key of elapsed time and value of response struct
	var currentBytePos int64
	var limit int64 = fileSize / 10 // sets limit of data chunk per goroutine by a tenth of the file size
	fmt.Println("limit", limit)
	// read all incoming matched words from channel and add to list
	go func() {
		for result := range matchedResults {
			if len(outputList) < 10 {
				dict[result.MatchedWord]++
				fmt.Println("res from channel", result)
				outputList = append(outputList, result)
			}
		}

		fmt.Print(dict)
		// let parent know that output list has been completed
		done <- true
	}()

	for i := 0; i < 10; i++ {
		waitGroup.Add(1)
		params := ScanFileParams{start, timeout, file, currentBytePos, limit, matchedResults}
		go scanFileForKeyword(params, &waitGroup)

		// increment byte pos by  lastbyte read by the prev thread + 1 (account for EOL)
		currentBytePos += limit + 1
	}

	waitGroup.Wait()
	close(matchedResults)
	writer.Flush()

	// wait for channel to process data
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

func scanFileForKeyword(input ScanFileParams, wg *sync.WaitGroup) {
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
			input.matchedResults <- failedToMatch
			return
		}

		if err != nil {
			// program errored out - failure
			input.matchedResults <- failedToMatch
			return
		}
	}

	var bytesReadSize int64
	for {
		// get elapsed time and check against timeout
		elapsed := getElapsedTime(input.start, "file-scan")
		if elapsed > input.timeout {
			fmt.Println("the process has timed out - elapsed:", elapsed)
			timedOutResponse := ResponseData{Status: TO}
			input.matchedResults <- timedOutResponse
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
			input.matchedResults <- failedToMatch
			return
		}

		removeSpecial := regexp.MustCompile(`(?m)[^a-z]`)
		bytesReadSize += int64(len(bytesRead))
		bytesToStr := strings.TrimSpace(string(bytesRead))
		lowercasedWord := strings.ToLower(string(bytesToStr))
		sanitizedWord := strings.Replace(lowercasedWord, "\n", "", -1)
		sanitizedWord = removeSpecial.ReplaceAllString(sanitizedWord, "")
		matchedKeyword, err := regexp.MatchString("\\bfico\\b", sanitizedWord)
		checkForError(err)
		elapsedForMatch := getElapsedTime(input.start, "matched")

		if lowercasedWord != "" && matchedKeyword {
			matchedData := ResponseData{Elapsed: int(elapsedForMatch), ByteCount: int(bytesReadSize), Status: Succ, MatchedWord: sanitizedWord}

			input.matchedResults <- matchedData
			return
		}
	}

	// if file chunk completed reading without a match, it's a fail

	input.matchedResults <- failedToMatch
}
