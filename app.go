package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/akamensky/argparse"
	"github.com/olekukonko/tablewriter"
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
	Elapsed   int64 // in ms
	ByteCount int64
	Status    string
}

type ScanFileParams struct {
	start          time.Time
	timeout        time.Duration
	file           *string
	offset         int64
	limit          int64
	matchedResults chan ResponseData
}

func main() {
	// set format of the logger to not output timestamp
	log.SetFlags(0)

	start := time.Now()
	timeout := 60 * 1000 * time.Millisecond

	parser := argparse.NewParser("fico-search", "This program takes in a file, reads and searches for the keyword 'fico' concurrently using go-routines.")
	file := parser.String("f", "file", &argparse.Options{Required: false, Help: "Pass in file to process."})
	timeoutArg := parser.Int("t", "timeout", &argparse.Options{Required: false, Help: "Pass in desired time in seconds for program timeout."})

	if timeoutArg != nil && *timeoutArg > 0 {
		timeout = time.Duration(int64(*timeoutArg)) * time.Millisecond
	}

	// fmt.Printf("file: %s, timeout: %d \n", *file, *timeoutArg)

	//!test
	s := parser.String("s", "string", &argparse.Options{Required: true, Help: "String to print"})
	fmt.Println(*s)
	// file := "./tmp/sampleInput.txt"
	// fileInput := os.Stdin

	fileInfo, err := os.Stat(*file)
	checkForError(err)
	fileSize := fileInfo.Size()
	fmt.Printf("the file is %d bytes long:\n", fileInfo.Size())

	matchedResults := make(chan (ResponseData)) // matchedResults used to scan the files for words in multiple goroutines
	done := make(chan (bool), 1)                // matchedResults to signal parent that data has been entered into dict
	var currentBytePos int64
	var limit int64 = fileSize / 10 // sets limit of data chunk per goroutine by a tenth of the file size
	fmt.Println("limit", limit)

	// read all incoming matched words from channel and add to list
	go func() {
		for result := range matchedResults {
			if len(outputList) < 10 {
				outputList = append(outputList, result)
			}
		}

		// let parent know that output list has been completed
		done <- true
	}()

	params := ScanFileParams{start, timeout, file, currentBytePos, limit, matchedResults}
	go addWorkersToWaitGroup(params)

	// wait for channel to process data
	<-done
	close(done)

	end := time.Now()
	log.Println("finished", end.Sub(start))

	printOutput(outputList)
}

func addWorkersToWaitGroup(params ScanFileParams) {
	waitGroup := sync.WaitGroup{} // waits for all goroutines to complete

	// create 10 go-routines and add to waitgroup
	for i := 0; i < 10; i++ {
		waitGroup.Add(1)
		go scanFileForKeyword(params, &waitGroup)

		// increment byte pos by  lastbyte read by the prev thread + 1 (account for EOL)
		params.offset += params.limit + 1
	}

	waitGroup.Wait()
	close(params.matchedResults)
}

func printOutput(outputList []ResponseData) {
	// sort by elapsed time desc
	sort.Slice(outputList, func(a, b int) bool {
		return outputList[a].Elapsed > outputList[b].Elapsed
	})

	dataToOutput := [][]string{}
	lineToAdd := []string{}
	var totalSumOfBytes float64 = 0
	var totalSumOfTimeInMs float64 = 0

	for _, output := range outputList {
		if output.Status == Succ {
			lineToAdd = []string{strconv.FormatInt(output.Elapsed, 10) + " ms", strconv.FormatInt(output.ByteCount, 10), output.Status}
		} else {
			lineToAdd = []string{"", "", output.Status}
		}

		totalSumOfBytes += float64(output.ByteCount)
		totalSumOfTimeInMs += float64(output.Elapsed)
		fmt.Println(totalSumOfTimeInMs)
		dataToOutput = append(dataToOutput, lineToAdd)
	}

	fmt.Println("totalSumOfBytes", totalSumOfBytes)
	fmt.Println("totalSumOfTimeInMs", totalSumOfTimeInMs)

	averageBytesPerMs := totalSumOfBytes / totalSumOfTimeInMs
	fmt.Println(averageBytesPerMs)
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"[elapsed]", "[byte_cnt]", "status]"})
	table.SetFooter([]string{"", "Average Bytes/ms", strconv.FormatFloat(averageBytesPerMs, 'f', 6, 64)})

	for _, line := range dataToOutput {
		switch line[2] {
		case Succ:
			table.Rich(line, []tablewriter.Colors{tablewriter.Colors{}, tablewriter.Colors{tablewriter.Normal, tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiGreenColor}, tablewriter.Colors{}})
		case TO:
			table.Rich(line, []tablewriter.Colors{tablewriter.Colors{}, tablewriter.Colors{tablewriter.Normal, tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.Bold, tablewriter.FgYellowColor}, tablewriter.Colors{}})
		case Fail:
			table.Rich(line, []tablewriter.Colors{tablewriter.Colors{}, tablewriter.Colors{tablewriter.Normal, tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.Bold, tablewriter.FgRedColor}, tablewriter.Colors{}})
		default:
			table.Append(line)
		}
	}
	table.Render()
}

func checkForError(err error) {
	if err != nil {
		// log defaults output to stdout: https://golang.org/src/log/log.go#L58
		log.Println(err)
	}
}

func getElapsedTime(start time.Time, name string) time.Duration {
	elapsed := time.Since(start) // time.Duration return is in nanoseconds
	return elapsed / 1000        // get ms
}

func scanFileForKeyword(input ScanFileParams, wg *sync.WaitGroup) {
	// Decreasing internal counter for wait-group as soon as goroutine finishes
	defer wg.Done()

	//early bail
	if len(outputList) > 10 {
		return
	}

	fileToRead, err := os.Open(*input.file)
	checkForError(err)
	defer fileToRead.Close()

	// move ptr to start of current chunk
	fileToRead.Seek(input.offset, 0)
	reader := bufio.NewReader(fileToRead)

	// if there is an offset, we need to shift bytes to the end of the word so the chunk starts at a new word
	if input.offset != 0 {
		_, err = reader.ReadBytes(' ')
		if err == io.EOF {
			log.Println(err)
			// EOF is a failure
			input.matchedResults <- failedToMatch
			return
		}

		if err != nil {
			// program errored out - failure
			input.matchedResults <- failedToMatch
			log.Println(err)
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
			log.Println(err)
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
			matchedData := ResponseData{Elapsed: int64(elapsedForMatch), ByteCount: int64(bytesReadSize), Status: Succ}
			input.matchedResults <- matchedData
			return
		}
	}

	// if file chunk completed reading without a match, it's a fail
	input.matchedResults <- failedToMatch
}
