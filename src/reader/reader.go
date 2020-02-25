package reader

import (
	"bufio"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dmdinh22/fico-search/helpers"
)

var failedToMatch = ResponseData{Status: Fail}

type ScannerByteCounter struct {
	BytesRead int
}

type ScanFileParams struct {
	Start          time.Time
	Timeout        time.Duration
	File           *string
	Offset         int64
	Limit          int64
	MatchedResults chan<- ResponseData
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

func ScanFileForKeyword(input ScanFileParams, wg *sync.WaitGroup) {
	// Decreasing internal counter for wait-group as soon as goroutine finishes
	defer wg.Done()

	fileToRead, err := os.Open(*input.File)
	helpers.CheckForError(err)
	defer fileToRead.Close()

	// move ptr to start of current chunk
	fileToRead.Seek(input.Offset, 0)
	reader := bufio.NewReader(fileToRead)

	// if there is an offset, we need to shift bytes to the end of the word so the chunk starts at a new word
	if input.Offset != 0 {
		_, err = reader.ReadBytes(' ')
		if err == io.EOF {
			log.Println(err)
			// EOF is a failure
			input.MatchedResults <- failedToMatch
			return
		}

		if err != nil {
			// program errored out - failure
			input.MatchedResults <- failedToMatch
			log.Println(err)
			return
		}
	}

	var bytesReadSize int64
	for {
		// get elapsed time and check against timeout
		elapsed := helpers.GetElapsedTime(input.Start, "file-scan")
		if time.Duration(elapsed) > input.Timeout {
			timedOutResponse := ResponseData{Status: TO}
			input.MatchedResults <- timedOutResponse
			return
		}

		// if the size has exceeded chunk limit, break out
		if bytesReadSize > input.Limit {
			break
		}

		bytesRead, err := reader.ReadBytes(' ')

		// break for end of file
		if err == io.EOF {
			break
		}

		if err != nil {
			input.MatchedResults <- failedToMatch
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
		elapsedForMatch := helpers.GetElapsedTime(input.Start, "matched")

		if lowercasedWord != "" && matchedKeyword {
			matchedData := ResponseData{Elapsed: int64(elapsedForMatch), ByteCount: int64(bytesReadSize), Status: Succ}
			input.MatchedResults <- matchedData
			return
		}
	}

	// if file chunk completed reading without a match, it's a fail
	input.MatchedResults <- failedToMatch
}
