package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const mb = 1024 * 1024
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

func main() {
	start := time.Now()
	timeout := 60 * 1000 * time.Millisecond //!TODO use 60s as def, 10s for testing for now

	fileInfo, err := os.Stat("./smallSample.txt")
	checkForError(err)
	fmt.Printf("the file is %d bytes long", fileInfo.Size())

	fileToScan, err := ioutil.ReadFile("./smallSample.txt")
	checkForError(err)
	writer := bufio.NewWriter(os.Stdout)
	buffedFile := bytes.NewReader(fileToScan)

	waitGroup := sync.WaitGroup{} // waits for all goroutines to complete
	channel := make(chan (string))
	var currentBytePos int64
	var limit int64 = 500 * mb

	for i := 0; i < 10; i++ {
		waitGroup.Add(1)
		fmt.Printf("thread %d started \n", i)

		go func() {
			scanFileForKeyword(start, timeout, buffedFile, currentBytePos, channel)
			fmt.Printf("%d thread completed \n", i)
			waitGroup.Done()
		}()
		// inc the byte pos by 1+ (lastbyte read by the prev thread)
		currentBytePos += limit + 1
	}

	waitGroup.Wait()
	close(channel)
	writer.Flush()

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
