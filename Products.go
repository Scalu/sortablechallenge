package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Result contains matching results to be exported
type Result struct {
	ProductName           string     `json:"product_name"`
	Listings              []*Listing `json:"listings"`
	tokenOrderDifferences []int
}

// Product defines the fields found in the products.txt json file
type Product struct {
	ProductName            string `json:"product_name"`
	Manufacturer           string `json:"manufacturer"`
	Model                  string `json:"model"`
	Family                 string `json:"family"`
	AnnouncedDate          string `json:"announced_date"`
	manufacturerTokenCount int
	familyTokenCount       int
	tokenList              []int
	result                 Result
}

// Products implements common interface for loading json data
type Products struct {
	products []*Product
}

// GetFileName used by JSONArchive.go
func (p *Products) GetFileName() string {
	return "products.txt"
}

// Decode used by JSONArchive.go
func (p *Products) Decode(decoder *json.Decoder) (err error) {
	product := &Product{}
	err = decoder.Decode(&product)
	if err == nil {
		product.result.ProductName = product.ProductName
		product.result.Listings = []*Listing{}
		p.products = append(p.products, product)
	}
	return
}

func generateTokensFromString(value string) (tokens []string) {
	tokenStart := 0
	value = strings.ToLower(value)
	var tokenIsNumeric bool
	for i := 0; i < len(value); i++ {
		if !(!tokenIsNumeric && value[i] >= 'a' && value[i] <= 'z' || tokenIsNumeric && value[i] >= '0' && value[i] <= '9') {
			if tokenIsNumeric && (value[i] == ',' || value[i] == '.') && len(value) > i+1 && value[i+1] >= '0' && value[i+1] <= '9' {
				continue
			}
			if i > tokenStart {
				tokens = append(tokens, value[tokenStart:i])
			}
			tokenStart = i
		}
		if value[i] >= 'a' && value[i] <= 'z' {
			tokenIsNumeric = false
		} else if value[i] >= '0' && value[i] <= '9' {
			tokenIsNumeric = true
		} else {
			tokenStart++
		}
	}
	if len(value) > tokenStart {
		tokens = append(tokens, value[tokenStart:])
	}
	return
}

// GetTokens returns a ProductTokens object initialized by the products
func (p *Products) GetTokens() (productTokens *ProductTokens) {
	productTokens = &ProductTokens{}
	for _, product := range p.products {
		tokenArray := []string{}
		tokenArray = append(tokenArray, generateTokensFromString(product.Manufacturer)...)
		product.manufacturerTokenCount = len(tokenArray)
		tokenArray = append(tokenArray, generateTokensFromString(product.Family)...)
		product.familyTokenCount = len(tokenArray) - product.manufacturerTokenCount
		tokenArray = append(tokenArray, generateTokensFromString(product.Model)...)
		product.tokenList = productTokens.AddTokens(product, tokenArray)
	}
	fmt.Println("Product tokens generated. ", len(p.products), " products, ", len(productTokens.tokens), " tokens")
	return
}

// GetProductCount returns the number of products in the array
func (p *Products) GetProductCount() int {
	return len(p.products)
}

func getWeightForTokenOrderDifference(tokenOrderDifference int) (weight int) {
	weight = 1
	if tokenOrderDifference < 6 {
		weight = (7 - tokenOrderDifference) * (7 - tokenOrderDifference)
	}
	return
}

func (p *Products) dropIrregularlyPricedResults() {
	// calculate the best range
	var bestRangeStartPrice, bestRangeMaxValue, bestRangeSpread float64
	var currentRangeStartPrice, currentRangeMaxValue, currentRangeSpread float64
	var secondListingPrice float64
	var bestRangeWeightValue, currentRangeWeightValue, listingIndex, secondIndex, totalWeight int
	var listing, secondListing *Listing
	var product *Product
	maxRangeSpread := 2.0
	for _, product = range p.products {
		if len(product.result.Listings) == 0 {
			continue
		}
		bestRangeStartPrice = 0.0
		bestRangeMaxValue = 0.0
		bestRangeSpread = 0.0
		bestRangeWeightValue = 0
		totalWeight = 0
		for listingIndex, listing = range product.result.Listings {
			currentRangeStartPrice = listing.GetPrice(-1.0)
			if currentRangeStartPrice < 0 {
				continue
			}
			currentRangeMaxValue = currentRangeStartPrice
			currentRangeWeightValue = 0
			totalWeight += getWeightForTokenOrderDifference(product.result.tokenOrderDifferences[listingIndex])
			for secondIndex, secondListing = range product.result.Listings {
				secondListingPrice = secondListing.GetPrice(-1.0)
				if secondListingPrice < 0 {
					continue
				}
				if secondListingPrice >= currentRangeStartPrice && secondListingPrice <= currentRangeStartPrice*maxRangeSpread {
					currentRangeWeightValue += getWeightForTokenOrderDifference(product.result.tokenOrderDifferences[secondIndex])
					if secondListingPrice > currentRangeMaxValue {
						currentRangeMaxValue = secondListingPrice
					}
				}
			}
			currentRangeSpread = currentRangeMaxValue / currentRangeStartPrice
			if currentRangeWeightValue > bestRangeWeightValue ||
				currentRangeWeightValue == bestRangeWeightValue && currentRangeSpread < bestRangeSpread {
				bestRangeStartPrice = currentRangeStartPrice
				bestRangeWeightValue = currentRangeWeightValue
				bestRangeMaxValue = currentRangeMaxValue
				bestRangeSpread = currentRangeSpread
			}
		}
		// remove all listings if weight value in spread is not high enough
		var listing *Listing
		if bestRangeWeightValue < totalWeight/2 {
			fmt.Println("Warning spread out pricing for product", product.ProductName)
			for _, listing = range product.result.Listings {
				listing.match = nil
			}
			product.result.Listings = []*Listing{}
			product.result.tokenOrderDifferences = []int{}
			continue
		}
		// drop listings the deviate too far out from the spread
		listingIndex := 0
		var allowedVariance, currentListingPrice float64
		var currentListingWeight int
		for listingIndex < len(product.result.Listings) {
			listing = product.result.Listings[listingIndex]
			currentListingPrice = listing.GetPrice(-1)
			currentListingWeight = getWeightForTokenOrderDifference(product.result.tokenOrderDifferences[listingIndex])
			allowedVariance = 1.0 + 0.05*float64(currentListingWeight)
			if currentListingPrice < bestRangeStartPrice/allowedVariance || currentListingPrice > bestRangeMaxValue*allowedVariance {
				listing.match = nil
				product.result.Listings = append(product.result.Listings[:listingIndex], product.result.Listings[listingIndex+1:]...)
				product.result.tokenOrderDifferences = append(product.result.tokenOrderDifferences[:listingIndex], product.result.tokenOrderDifferences[listingIndex+1:]...)
			} else {
				listingIndex++
			}
		}
	}
}

func (p *Products) exportResults(filename string) {
	resultsFile, err := os.Create(filename)
	if err != nil {
		fmt.Println("Error creating file for unmatched listings:", err)
		os.Exit(1)
	}
	defer resultsFile.Close()
	jsonEncoder := json.NewEncoder(resultsFile)
	for _, product := range p.products {
		err = jsonEncoder.Encode(product.result)
		if err != nil {
			fmt.Println("Error exporting results to file", filename, ":", err)
			os.Exit(1)
		}
	}
}
