package main

import (
	"fmt"
	"os"
	"time"

	"github.com/Scalu/sortablechallenge/originalmatcher"
	"github.com/Scalu/sortablechallenge/sortablechallengeutils"
)

func main() {
	startTime := time.Now()
	fmt.Println("Begining sortedchallenge program at", startTime)
	defer func(startTime time.Time) {
		fmt.Printf("Exiting sortedchallenge. Duration: %s\n", time.Since(startTime))
	}(startTime)
	dh := dataHandler{
		matchers: []matcher{&originalmatcher.OriginalMatcher{}},
		archive: sortablechallengeutils.JSONArchive{
			ArchiveFileName:  "challenge_data_20110429.tar.gz",
			ArchiveSourceURL: "https://s3.amazonaws.com/sortable-public/challenge/challenge_data_20110429.tar.gz"},
		osExit:   os.Exit,
		osCreate: os.Create}
	dh.run()
}
