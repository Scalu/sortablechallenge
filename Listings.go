package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Listing defines the fields found in the listings.txt json file
type Listing struct {
	Title        string `json:"title"`
	Manufacturer string `json:"manufacturer"`
	Currency     string `json:"currency"`
	Price        string `json:"price"`
	match        *Product
}

// GetPrice return price of item in USD
func (l *Listing) GetPrice(defaultPrice float64) float64 {
	price, err := strconv.ParseFloat(l.Price, 32)
	if err != nil {
		fmt.Println("Price conversion error for listing", l, err)
		return defaultPrice
	}
	switch strings.ToLower(l.Currency) {
	case "usd":
		return price
	case "cad":
		return price / 1.34
	case "eur":
		return price / 0.92
	case "gbp":
		return price / 0.79
	}
	fmt.Println("Unhandled currency for listing", l)
	return defaultPrice
}

// Listings struct to hold the listing data
// implementes JSONDecoder
type Listings struct {
	listings []*Listing
}

// GetFileName used by JSONArchive util
func (l *Listings) GetFileName() string {
	return "listings.txt"
}

// Decode used by JSONArchive util
func (l *Listings) Decode(decoder *json.Decoder) (err error) {
	listing := &Listing{}
	err = decoder.Decode(&listing)
	if err == nil {
		l.listings = append(l.listings, listing)
	}
	return
}

// GetListingsCount returns the number of listings
func (l *Listings) GetListingsCount() int {
	return len(l.listings)
}

func isSubsetOf(possibleSubset, possibleSuperset []int) bool {
	if len(possibleSubset) >= len(possibleSuperset) {
		return false
	}
	for _, subsetValue := range possibleSubset {
		matchFound := false
		for _, supersetValue := range possibleSuperset {
			if subsetValue == supersetValue {
				matchFound = true
				break
			}
		}
		if !matchFound {
			return false
		}
	}
	return true
}

func addPossibleMatch(pt *ProductTokens, possibleMatches *[]*Product, tokenOrderDifferences *[]int, listingTokens []string, possibleMatch *Product) {
	// don't add the product if it's already in the possible matches
	for _, existingMatch := range *possibleMatches {
		if existingMatch == possibleMatch {
			return
		}
	}
	// make sure that all of the tokens are present, and calculate the token order difference
	tokenOrderDifference := 0
	expectedNextTokenPosition := 0
	missingManufacturerTokens := false
	missingFamilyTokens := false
	for tokenIndex, tokenObjectIndex := range possibleMatch.tokenList {
		requiredToken := &pt.tokens[tokenObjectIndex]
		tokenFound := false
		for distanceFromExpected := 0; distanceFromExpected <= expectedNextTokenPosition || distanceFromExpected+expectedNextTokenPosition < len(listingTokens); distanceFromExpected++ {
			if distanceFromExpected+expectedNextTokenPosition < len(listingTokens) {
				listingToken := listingTokens[expectedNextTokenPosition+distanceFromExpected]
				if listingToken == requiredToken.value {
					if distanceFromExpected <= 2 ||
						tokenIndex < possibleMatch.manufacturerTokenCount ||
						tokenIndex >= possibleMatch.manufacturerTokenCount+possibleMatch.familyTokenCount {
						tokenFound = true
						tokenOrderDifference += distanceFromExpected
						expectedNextTokenPosition = expectedNextTokenPosition + distanceFromExpected + 1
						break
					}
				}
			}
			if distanceFromExpected+1 < expectedNextTokenPosition && distanceFromExpected > 0 {
				listingToken := listingTokens[expectedNextTokenPosition-1-distanceFromExpected]
				if listingToken == requiredToken.value {
					tokenFound = true
					tokenOrderDifference += distanceFromExpected
					expectedNextTokenPosition = expectedNextTokenPosition - distanceFromExpected
					break
				}
			}
		}
		if !tokenFound {
			if tokenIndex < possibleMatch.manufacturerTokenCount {
				if !missingFamilyTokens {
					if !missingManufacturerTokens {
						missingManufacturerTokens = true
						tokenOrderDifference += 2
					}
					continue
				}
			} else if tokenIndex < possibleMatch.manufacturerTokenCount+possibleMatch.familyTokenCount {
				if !missingManufacturerTokens {
					if !missingFamilyTokens {
						missingFamilyTokens = true
						tokenOrderDifference += 2
					}
					continue
				}
			}
			return
		}
	}
	// eliminate subsets of this product, and eliminate this product if it's a subset of an existing product
	for existingIndex, existingMatch := range *possibleMatches {
		if isSubsetOf(possibleMatch.tokenList, existingMatch.tokenList) {
			return
		}
		if isSubsetOf(existingMatch.tokenList, possibleMatch.tokenList) &&
			tokenOrderDifference <= (*tokenOrderDifferences)[existingIndex] {
			*possibleMatches = append((*possibleMatches)[:existingIndex], (*possibleMatches)[existingIndex+1:]...)
			*tokenOrderDifferences = append((*tokenOrderDifferences)[:existingIndex], (*tokenOrderDifferences)[existingIndex+1:]...)
		}
	}
	*possibleMatches = append(*possibleMatches, possibleMatch)
	*tokenOrderDifferences = append(*tokenOrderDifferences, tokenOrderDifference)
}

// MapToProducts associates listings with products
func (l *Listings) MapToProducts(pt *ProductTokens) {
	// get a list of matching tokens and possible matches
	for _, listing := range l.listings {
		possibleMatches := []*Product{}
		tokenOrderDifferences := []int{}
		listingTokens := generateTokensFromString(listing.Title)
		for _, listingToken := range listingTokens {
			matchingToken := pt.getMatchingToken(listingToken)
			if matchingToken == nil {
				continue
			}
			for _, matchingProduct := range matchingToken.products {
				addPossibleMatch(pt, &possibleMatches, &tokenOrderDifferences, listingTokens, matchingProduct)
			}
		}
		// eliminate a match with multiple products with tokenOrderDifferences that are close in value
		// set the match of the token order difference is below the threshhold
		var matchedProduct *Product
		bestTokenOrderDifference := 50
		var tokenOrderDifference int
		for possibleIndex, possibleProduct := range possibleMatches {
			if possibleProduct != nil {
				tokenOrderDifference = tokenOrderDifferences[possibleIndex]
				if matchedProduct != nil {
					if tokenOrderDifference*2 < bestTokenOrderDifference {
						bestTokenOrderDifference = tokenOrderDifference
						matchedProduct = possibleProduct
						continue
					}
					if tokenOrderDifference < bestTokenOrderDifference*2 {
						matchedProduct = nil
						break
					}
					continue
				}
				if tokenOrderDifference > 2+len(possibleProduct.tokenList) {
					continue
				}
				bestTokenOrderDifference = tokenOrderDifference
				matchedProduct = possibleProduct
			}
		}
		if matchedProduct != nil {
			listing.match = matchedProduct
			matchedProduct.result.Listings = append(matchedProduct.result.Listings, listing)
			matchedProduct.result.tokenOrderDifferences = append(matchedProduct.result.tokenOrderDifferences, bestTokenOrderDifference)
		}
	} // end of iterating through listings
}

func (l *Listings) exportUnmatchedListings(filename string) {
	unmatchedListingsFile, err := os.Create(filename)
	if err != nil {
		fmt.Println("Error creating file for unmatched listings:", err)
		os.Exit(1)
	}
	defer unmatchedListingsFile.Close()
	jsonEncoder := json.NewEncoder(unmatchedListingsFile)
	for _, listing := range l.listings {
		if listing.match == nil {
			err = jsonEncoder.Encode(listing)
			if err != nil {
				fmt.Println("Error exporting unmatched listings to file", filename, ":", err)
				os.Exit(1)
			}
		}
	}
}
