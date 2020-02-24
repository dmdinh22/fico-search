package helpers

import (
	"log"
	"time"
)

func CheckForError(err error) {
	if err != nil {
		// log defaults output to stdout: https://golang.org/src/log/log.go#L58
		log.Println(err)
	}
}

func GetElapsedTime(start time.Time, name string) time.Duration {
	elapsed := time.Since(start) // time.Duration return is in nanoseconds
	return elapsed / 1000        // get ms
}
