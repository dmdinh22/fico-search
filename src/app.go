package main

import (
	"flag"
	"log"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/akamensky/argparse"
	"github.com/dmdinh22/fico-search/helpers"
	"github.com/dmdinh22/fico-search/reader"
	"github.com/olekukonko/tablewriter"
)

var outputList []reader.ResponseData

func main() {
	// set format of the logger to not output timestamp
	log.SetFlags(0)
	start := time.Now()
	timeout := 60 * time.Millisecond

	parser := argparse.NewParser("fico-search", "This program takes in a file, reads and searches for the keyword 'fico' concurrently using go-routines.")
	file := parser.String("f", "file", &argparse.Options{Required: true, Help: "Flag to pass arg with path of a file to process."})
	timeoutArg := parser.Int("t", "timeout", &argparse.Options{Required: false, Help: "Flag to pass in desired time in seconds for program timeout. (default 60s)"})

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

	waitGroup := sync.WaitGroup{}                    // waits for all goroutines to complete
	matchedResults := make(chan reader.ResponseData) // matchedResults used to scan the files for words in multiple goroutines
	completed := make(chan (bool), 1)                // matchedResults to signal parent that data has been entered into dict
	var currentBytePos int64
	var limit int64 = fileSize / 10 // sets limit of data chunk per goroutine by a tenth of the file size

	params := reader.ScanFileParams{
		Start:          start,
		Timeout:        timeout,
		File:           file,
		Offset:         currentBytePos,
		Limit:          limit,
		MatchedResults: matchedResults,
	}

	for i := 0; i < 10; i++ {
		waitGroup.Add(1)
		go reader.ScanFileForKeyword(params, &waitGroup)

		// increment byte pos by  lastbyte read by the prev thread + 1 (account for EOL)
		params.Offset += params.Limit + 1
	}

	go channelMonitor(&waitGroup, matchedResults)

	// read all incoming matched words from channel and add to list
	go func() {
		for result := range matchedResults {
			if len(outputList) < 10 {
				outputList = append(outputList, result)
			}
		}

		// let parent know that output list has been completed
		completed <- true
	}()

	// wait for completed channel to process data
	<-completed
	close(completed)

	end := time.Now()
	endTime := end.Sub(start) / 1000 / 1000 // convert to ms
	log.Printf("File search completed in %d ms.", endTime)

	printOutput(outputList)
}

func channelMonitor(waitGroup *sync.WaitGroup, responseChanel chan reader.ResponseData) {
	waitGroup.Wait()
	close(responseChanel)
}

func printOutput(outputList []reader.ResponseData) {
	// sort by elapsed time desc
	sort.Slice(outputList, func(a, b int) bool {
		return outputList[a].Elapsed > outputList[b].Elapsed
	})

	dataToOutput := [][]string{}
	lineToAdd := []string{}
	var totalSumOfBytes float64 = 0
	var totalSumOfTimeInMs float64 = 0

	for _, output := range outputList {
		if output.Status == reader.Succ {
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
		case reader.Succ:
			table.Rich(line, []tablewriter.Colors{tablewriter.Colors{}, tablewriter.Colors{tablewriter.Normal, tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiGreenColor}, tablewriter.Colors{}})
		case reader.TO:
			table.Rich(line, []tablewriter.Colors{tablewriter.Colors{}, tablewriter.Colors{tablewriter.Normal, tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.Bold, tablewriter.FgYellowColor}, tablewriter.Colors{}})
		case reader.Fail:
			table.Rich(line, []tablewriter.Colors{tablewriter.Colors{}, tablewriter.Colors{tablewriter.Normal, tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.Bold, tablewriter.FgRedColor}, tablewriter.Colors{}})
		default:
			table.Append(line)
		}
	}
	table.Render()
}
