package worker

import (
	"bufio"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"../helpers"
)

type ResponseData struct {
	Elapsed   int64 // in ms
	ByteCount int64
	Status    string
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
		if time.Duration(elapsed) > input.timeout {
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
