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
	"time"
)

type ScannerByteCounter struct {
	BytesRead int
}

type ResponseData struct {
	TimeElapsed int
	BytesRead   int
	Status      string
}

func main() {
	start := time.Now()
	timeout := 60 * 1000 * time.Millisecond

	fileInfo, err := os.Stat("./smallSample.txt")
	checkForError(err)
	fmt.Printf("the file is %d bytes long", fileInfo.Size())

	fileToScan, err := ioutil.ReadFile("./smallSample.txt")
	checkForError(err)
	writer := bufio.NewWriter(os.Stdout)
	buffedFile := bytes.NewReader(fileToScan)

	scanFileForKeyword(start, timeout, buffedFile)

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
	// log.Printf("%s took %s", name, elapsed)

	return elapsed
}

func scanFileForKeyword(start time.Time, timeout time.Duration, file *bytes.Reader) {
	byteCtr := ScannerByteCounter{}
	scannedFile := bufio.NewScanner(file)
	splitFunc := byteCtr.Wrap(bufio.ScanWords)
	scannedFile.Split(splitFunc)

	for scannedFile.Scan() {
		elapsed := getElapsedTime(start, "file-scan")

		if elapsed > timeout {
			fmt.Println("the process has timed out - elapsed:", elapsed)
			log.Fatal("timed out...")
		}

		word := scannedFile.Bytes()
		wordAsString := strings.ToLower(string(word))
		matchedKeyword, err := regexp.MatchString("\\bfico\\b", wordAsString)
		checkForError(err)

		if matchedKeyword {
			elapsedForMatch := getElapsedTime(start, "matched")
			fmt.Println(wordAsString)
			fmt.Printf("Split Text: %s\n", scannedFile.Text())
			fmt.Printf("Bytes Read: %d\n\n", byteCtr.BytesRead)
			matchedData := ResponseData{TimeElapsed: int(elapsedForMatch), BytesRead: byteCtr.BytesRead, Status: "Success!"}

			fmt.Print(matchedData)
		}
	}
	if err := scannedFile.Err(); err != nil {
		log.Fatal(err)
	}
}

// SplitFunc returns amt of bytes forward scanner advances
func (byteCtr *ScannerByteCounter) Wrap(split bufio.SplitFunc) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (int, []byte, error) {
		bytesToAdvance, token, err := split(data, atEOF)
		byteCtr.BytesRead += bytesToAdvance
		return bytesToAdvance, token, err
	}
}
