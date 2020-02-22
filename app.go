package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const kb = 1024
const mb = 1024 * kb
const gb = 1024 * mb

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
	Elapsed   int
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
	start := time.Now()
	timeout := 60 * 1000 * time.Millisecond //!TODO use 60s as def, 10s for testing for now
	file := "./smallSample.txt"

	fileInfo, err := os.Stat(file)
	checkForError(err)
	fmt.Printf("the file is %d bytes long", fileInfo.Size())
	writer := bufio.NewWriter(os.Stdout)

	channel := make(chan (string)) // channel used to scan the files for words in multiple goroutines
	waitGroup := sync.WaitGroup{}  // waits for all goroutines to complete
	// channel := make(chan (string)) // channel used to scan the files for words in multiple goroutines
	dict := make(map[string]int) // stores key of elapsed time and value of response struct
	done := make(chan (bool), 1) // channel to signal parent that data has been entered into dict

	// read all incoming words from channel and count
	go func() {
		for str := range channel {
			//TODO: make map of time and data
			// fmt.Print("response from channel", response)
			dict[str]++
		}

		// let parent know that dict has been updated
		done <- true
	}()

	var currentBytePos int64
	var limit int64 = 5 * kb // sets limit of data chunk per goroutine

	for i := 0; i < 10; i++ {
		waitGroup.Add(1)
		fmt.Printf("thread %d started \n", i)

		go func() {
			params := ScanFileParams{start, timeout, file, currentBytePos, limit, channel}
			go scanFileForKeyword(params)

			fmt.Printf("thread %d completed \n", i)
			waitGroup.Done()
		}()
		// increment byte pos by 1 + (lastbyte read by the prev thread)
		currentBytePos += limit + 1
	}

	waitGroup.Wait()
	close(channel)
	writer.Flush()

	// wait for dict to process data
	<-done
	close(done)

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

func scanFileForKeyword(start time.Time, timeout time.Duration, file *bytes.Reader, offset int64, channel chan (string)) {
	byteCtr := ScannerByteCounter{}
	scannedFile := bufio.NewScanner(file)
	splitFunc := byteCtr.Wrap(bufio.ScanWords)
	scannedFile.Split(splitFunc)

	for scannedFile.Scan() {
		elapsed := getElapsedTime(start, "file-scan")

		if elapsed > timeout {
			fmt.Println("the process has timed out - elapsed:", elapsed)
			timedOutResponse := ResponseData{Status: TO}
			fmt.Print(timedOutResponse)
			log.Fatal("timed out...")
			return
		}

		word := scannedFile.Bytes()
		wordAsString := strings.ToLower(string(word))
		matchedKeyword, err := regexp.MatchString("\\bfico\\b", wordAsString)
		checkForError(err)

		if matchedKeyword {
			elapsedForMatch := getElapsedTime(start, "matched")
			// fmt.Println(wordAsString)
			// fmt.Printf("Split Text: %s\n", scannedFile.Text())
			// fmt.Printf("Bytes Read: %d\n\n", byteCtr.BytesRead)
			matchedData := ResponseData{Elapsed: int(elapsedForMatch), ByteCount: byteCtr.BytesRead, Status: Succ}

			fmt.Print(matchedData)
			//TODO: add to list to log out later
		}
	}

	if err := scannedFile.Err(); err != nil {
		log.Fatal(err)
	}

	failedToMatch := ResponseData{Status: Fail}
	fmt.Print(failedToMatch)
}

// SplitFunc returns amt of bytes forward scanner advances
func (byteCtr *ScannerByteCounter) Wrap(split bufio.SplitFunc) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (int, []byte, error) {
		bytesToAdvance, token, err := split(data, atEOF)
		byteCtr.BytesRead += bytesToAdvance
		return bytesToAdvance, token, err
	}
}
