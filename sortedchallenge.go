package main

import (
	"fmt"
	"os"
	"time"

	"github.com/Scalu/sortedchallenge/sortedchallengeutils"
)

func main() {
	startTime := time.Now()
	fmt.Println("Begining sortedchallenge program at ", startTime)
	defer fmt.Println("Exiting sortedchallenge. Duration: ", time.Since(startTime))
	// load the products and listings data
	products := sortedchallengeutils.Products{}
	listings := sortedchallengeutils.Listings{}
	archive := sortedchallengeutils.JSONArchive{ArchiveFileName: "challenge_data_20110429.tar.gz", ArchiveSourceURL: "https://s3.amazonaws.com/sortable-public/challenge/challenge_data_20110429.tar.gz"}
	err := archive.ImportJSONFromArchiveFile(products)
	if err != nil {
		fmt.Println("Error importing products data: ", err)
		os.Exit(1)
	}
	err = archive.ImportJSONFromArchiveFile(listings)
	if err != nil {
		fmt.Println("Error importing listings data: ", err)
		os.Exit(1)
	}
	fmt.Println("Done loading JSON data")
	// generate product signatures
	productSignatures := products.GetSignatures()
	// map listings to signatures
	// output results
}
