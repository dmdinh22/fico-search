package main

import (
	"bufio"
	"flag"
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
	"github.com/dmdinh22/fico-search/helpers"
	"github.com/olekukonko/tablewriter"
)

var outputList []ResponseData
var failedToMatch = ResponseData{Status: Fail}

type ScanFileParams struct {
	start          time.Time
	timeout        time.Duration
	file           *string
	offset         int64
	limit          int64
	matchedResults chan ResponseData
}

type ResponseData struct {
	Elapsed   int64 // in ms
	ByteCount int64
	Status    string
}

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

func main() {
	// set format of the logger to not output timestamp
	log.SetFlags(0)
	start := time.Now()
	timeout := 60 * time.Millisecond

	parser := argparse.NewParser("fico-search", "This program takes in a file, reads and searches for the keyword 'fico' concurrently using go-routines.")
	file := parser.String("f", "file", &argparse.Options{Required: true, Help: "Flag to pass arg with path of a file to process."})
	timeoutArg := parser.Int("t", "timeout", &argparse.Options{Required: false, Help: "Flag to pass in desired time in seconds for program timeout."})

	// parse input from args
	err := parser.Parse(os.Args)
	helpers.CheckForError(err)

	if *file == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if timeoutArg != nil && *timeoutArg > 0 {
		timeout = time.Duration(int64(*timeoutArg)) * time.Millisecond // convert to ms
	}

	log.Printf("Started to process file at %s.", start)
	fileInfo, err := os.Stat(*file)
	helpers.CheckForError(err)
	fileSize := fileInfo.Size()

	matchedResults := make(chan (ResponseData)) // matchedResults used to scan the files for words in multiple goroutines
	done := make(chan (bool), 1)                // matchedResults to signal parent that data has been entered into dict
	var currentBytePos int64
	var limit int64 = fileSize / 10 // sets limit of data chunk per goroutine by a tenth of the file size

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
	endTime := end.Sub(start) / 1000 / 1000 // convert to ms
	log.Printf("File search completed in %d ms.", endTime)

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

func scanFileForKeyword(input ScanFileParams, wg *sync.WaitGroup) {
	// Decreasing internal counter for wait-group as soon as goroutine finishes
	defer wg.Done()

	//early bail
	if len(outputList) > 10 {
		return
	}

	fileToRead, err := os.Open(*input.file)
	helpers.CheckForError(err)
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
		elapsed := helpers.GetElapsedTime(input.start, "file-scan")
		if elapsed > input.timeout {
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
		// CheckForError(err)
		elapsedForMatch := helpers.GetElapsedTime(input.start, "matched")

		if lowercasedWord != "" && matchedKeyword {
			matchedData := ResponseData{Elapsed: int64(elapsedForMatch), ByteCount: int64(bytesReadSize), Status: Succ}
			input.matchedResults <- matchedData
			return
		}
	}

	// if file chunk completed reading without a match, it's a fail
	input.matchedResults <- failedToMatch
}

func ScanFileForKeyword(input ScanFileParams, wg *sync.WaitGroup) {
	// Decreasing internal counter for wait-group as soon as goroutine finishes
	defer wg.Done()

	//early bail
	if len(outputList) > 10 {
		return
	}

	fileToRead, err := os.Open(*input.file)
	helpers.CheckForError(err)
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
		elapsed := helpers.GetElapsedTime(input.start, "file-scan")
		if elapsed > input.timeout {
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
		helpers.CheckForError(err)
		elapsedForMatch := helpers.GetElapsedTime(input.start, "matched")

		if lowercasedWord != "" && matchedKeyword {
			matchedData := ResponseData{Elapsed: int64(elapsedForMatch), ByteCount: int64(bytesReadSize), Status: Succ}
			input.matchedResults <- matchedData
			return
		}
	}

	// if file chunk completed reading without a match, it's a fail
	input.matchedResults <- failedToMatch
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
		dataToOutput = append(dataToOutput, lineToAdd)
	}
	averageBytesPerMs := totalSumOfBytes / totalSumOfTimeInMs

	// build output table for stdout
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
