package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Scalu/sortablechallenge/sortablechallengeutils"
)

type matcher interface {
	GetResults(products []sortablechallengeutils.Product, listings []sortablechallengeutils.Listing) []sortablechallengeutils.Result
	GetID() string
}

type dataHandler struct {
	products []sortablechallengeutils.Product
	listings []sortablechallengeutils.Listing
	matchers []matcher
	archive  sortablechallengeutils.JSONArchive
	osExit   func(int)
	osCreate func(string) (*os.File, error)
}

func (dh *dataHandler) loadData() {
	var product sortablechallengeutils.Product
	err := dh.archive.ImportJSONFromArchiveFile("products.txt", func(decoder interface {
		Decode(interface{}) error
	}) error {
		err := decoder.Decode(&product)
		if err == nil {
			dh.products = append(dh.products, product)
		}
		return err
	})
	if err != nil {
		fmt.Println("Error importing products data:", err)
		dh.osExit(1)
	}
	var listing sortablechallengeutils.Listing
	err = dh.archive.ImportJSONFromArchiveFile("listings.txt", func(decoder interface {
		Decode(interface{}) error
	}) error {
		err := decoder.Decode(&listing)
		if err == nil {
			dh.listings = append(dh.listings, listing)
		}
		return err
	})
	if err != nil {
		fmt.Println("Error importing listings data:", err)
		dh.osExit(1)
	}
	fmt.Println("Done loading JSON data.", len(dh.products), "products,", len(dh.listings), "listings")
}

func (dh *dataHandler) createFileAndJSONEncoder(fileName string) (*os.File, *json.Encoder) {
	file, err := dh.osCreate(fileName)
	if err != nil {
		fmt.Println("Error creating file:", err)
		dh.osExit(1)
	}
	jsonEncoder := json.NewEncoder(file)
	return file, jsonEncoder
}

func (dh *dataHandler) run() {
	var results []sortablechallengeutils.Result
	var err error
	dh.loadData()
	for _, matcher := range dh.matchers {
		matcherStartTime := time.Now()
		fmt.Printf("Running matcher %s at %s\n", matcher.GetID(), matcherStartTime)
		results = matcher.GetResults(dh.products, dh.listings)
		fmt.Printf("Done running matcher %s. Duration: %s\n", matcher.GetID(), time.Since(matcherStartTime))
		resultsFilename := "results.txt"
		unmatchedFilename := "unmatched.txt"
		if len(dh.matchers) > 1 {
			resultsFilename = fmt.Sprintf("results_%s.txt", matcher.GetID())
			unmatchedFilename = fmt.Sprintf("unmatched_%s.txt", matcher.GetID())
		}
		resultsFile, resultsEncoder := dh.createFileAndJSONEncoder(resultsFilename)
		defer resultsFile.Close()
		unmatchedFile, unmatchedEncoder := dh.createFileAndJSONEncoder(unmatchedFilename)
		defer unmatchedFile.Close()
		matchedCount, unmatchedCount := 0, 0
		for _, result := range results {
			if result.ProductName == "" {
				err = resultsEncoder.Encode(&result)
				if err != nil {
					fmt.Println("Error writing result to file", resultsFilename, ":", err)
					dh.osExit(1)
				}
				matchedCount += len(result.Listings)
			} else {
				for _, listing := range result.Listings {
					err = unmatchedEncoder.Encode(listing)
					if err != nil {
						fmt.Println("Error writing unmatched listing to file", unmatchedFilename, ":", err)
						dh.osExit(1)
					}
					unmatchedCount++
				}
			}
		}
		fmt.Printf("%s: number of matched listings: %d, unmatched listings: %d\n", matcher.GetID(), matchedCount, unmatchedCount)
	}
}
