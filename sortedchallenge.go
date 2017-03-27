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
	products := Products{}
	listings := Listings{}
	archive := sortedchallengeutils.JSONArchive{ArchiveFileName: "challenge_data_20110429.tar.gz", ArchiveSourceURL: "https://s3.amazonaws.com/sortable-public/challenge/challenge_data_20110429.tar.gz"}
	err := archive.ImportJSONFromArchiveFile(&products)
	if err != nil {
		fmt.Println("Error importing products data: ", err)
		os.Exit(1)
	}
	err = archive.ImportJSONFromArchiveFile(&listings)
	if err != nil {
		fmt.Println("Error importing listings data: ", err)
		os.Exit(1)
	}
	fmt.Println("Done loading JSON data. ", products.GetProductCount(), " products, ", listings.GetListingsCount(), " listings")
	// generate product signatures
	productTokens := products.GetTokens()
	// map listings to signatures
	listings.MapToProducts(productTokens)
	// weed out price abberations
	products.dropIrregularlyPricedResults()
	// export results
	listings.exportUnmatchedListings("unmatched.txt")
	products.exportResults("results.txt")
}
