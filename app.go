package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"
)

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
	log.Printf("%s took %s", name, elapsed)

	return elapsed
}

func scanFileForKeyword(start time.Time, timeout time.Duration, file *bytes.Reader) {
	scannedFile := bufio.NewScanner(file)
	scannedFile.Split(bufio.ScanWords)
	for scannedFile.Scan() {
		elapsed := getElapsedTime(start, "file-scan")

		if elapsed > timeout {
			fmt.Println("the process has timed out - elapsed:", elapsed)
			log.Fatal("timed out...")
		}

		word := scannedFile.Bytes()
		wordAsString := strings.ToLower(string(word))
		if strings.Contains(wordAsString, "fico") {
			fmt.Println(wordAsString)
		}
	}
	if err := scannedFile.Err(); err != nil {
		log.Fatal(err)
	}
}
