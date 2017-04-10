package originalmatcher

import (
	"fmt"
	"time"

	"github.com/Scalu/sortablechallenge/sortablechallengeutils"
)

// OriginalMatcher the original matching algorithm for this challenge
type OriginalMatcher struct {
	products Products
	listings Listings
}

// GetID returns this matcher's ID
func (om *OriginalMatcher) GetID() string {
	return "originalMatcher"
}

// GetResults returns an array of results produced by the original algorithm
func (om *OriginalMatcher) GetResults(products []sortablechallengeutils.Product, listings []sortablechallengeutils.Listing) (results []sortablechallengeutils.Result) {
	// map generic products and listings to original ones
	for _, product := range products {
		om.products.products = append(om.products.products, &originalProduct{
			ProductName:   product.ProductName,
			Model:         product.Model,
			Manufacturer:  product.Manufacturer,
			Family:        product.Family,
			AnnouncedDate: product.AnnouncedDate,
			result:        originalResult{ProductName: product.ProductName}})
	}
	for _, listing := range listings {
		om.listings.listings = append(om.listings.listings, &originalListing{
			Currency:     listing.Currency,
			Price:        listing.Price,
			Title:        listing.Title,
			Manufacturer: listing.Manufacturer})
	}
	// keep track of time not including the mapping of the values for a fair comparison with new matchers
	matcherStartTime := time.Now()
	// generate product signatures
	productTokens := om.products.GetTokens()
	// map listings to signatures
	om.listings.MapToProducts(productTokens)
	// weed out price abberations
	om.products.dropIrregularlyPricedResults()
	fmt.Printf("Done running original matcher %s. Duration without additional mapping: %s\n", om.GetID(), time.Since(matcherStartTime))
	// transfer originalResults to general results
	results = []sortablechallengeutils.Result{}
	for _, product := range om.products.products {
		results = append(results, sortablechallengeutils.Result{ProductName: product.ProductName})
		for _, listing := range product.result.Listings {
			results[len(results)-1].Listings = append(results[len(results)-1].Listings, &sortablechallengeutils.Listing{
				Currency:     listing.Currency,
				Price:        listing.Price,
				Title:        listing.Title,
				Manufacturer: listing.Manufacturer})
		}
	}
	// transfer unmatched results to general results
	results = append(results, sortablechallengeutils.Result{})
	for _, listing := range om.listings.listings {
		if listing.match == nil {
			results[len(results)-1].Listings = append(results[len(results)-1].Listings, &sortablechallengeutils.Listing{
				Currency:     listing.Currency,
				Price:        listing.Price,
				Title:        listing.Title,
				Manufacturer: listing.Manufacturer})
		}
	}
	return results
}
