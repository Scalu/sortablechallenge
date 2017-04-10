package originalmatcher

import (
	"encoding/json"
	"fmt"
	"os"
)

// originalResult contains matching results to be exported
type originalResult struct {
	ProductName           string             `json:"product_name"`
	Listings              []*originalListing `json:"listings"`
	tokenOrderDifferences []int
}

// originalProduct defines the fields found in the products.txt json file
type originalProduct struct {
	ProductName            string `json:"product_name"`
	Manufacturer           string `json:"manufacturer"`
	Model                  string `json:"model"`
	Family                 string `json:"family"`
	AnnouncedDate          string `json:"announced_date"`
	manufacturerTokenCount int
	familyTokenCount       int
	tokenList              []int
	result                 originalResult
}

// Products implements common interface for loading json data
type Products struct {
	products            []*originalProduct
	matchedProductCount int
}

// GetFileName used by JSONArchive.go
func (p *Products) GetFileName() string {
	return "products.txt"
}

// Decode used by JSONArchive.go
func (p *Products) Decode(decoder *json.Decoder) (err error) {
	product := &originalProduct{}
	err = decoder.Decode(&product)
	if err == nil {
		product.result.ProductName = product.ProductName
		product.result.Listings = []*originalListing{}
		p.products = append(p.products, product)
	}
	return
}

// GetTokens returns a ProductTokens object initialized by the products
func (p *Products) GetTokens() (productTokens *ProductTokens) {
	productTokens = &ProductTokens{}
	for _, product := range p.products {
		tokenArray := generateTokensFromString(product.Manufacturer)
		product.manufacturerTokenCount = len(tokenArray)
		product.tokenList = append(product.tokenList, productTokens.AddTokens(nil, tokenArray)...)
		tokenArray = generateTokensFromString(product.Family)
		product.familyTokenCount = len(tokenArray)
		product.tokenList = append(product.tokenList, productTokens.AddTokens(nil, tokenArray)...)
		tokenArray = generateTokensFromString(product.Model)
		product.tokenList = append(product.tokenList, productTokens.AddTokens(product, tokenArray)...)
	}
	fmt.Println("Product tokens generated. ", len(p.products), " products, ", len(productTokens.tokens), " tokens")
	return
}

// GetProductCount returns the number of products in the array
func (p *Products) GetProductCount() int {
	return len(p.products)
}

// getWeightForTokenOrderDifference gets a weight value based on the given tokenOrderDifference
func getWeightForTokenOrderDifference(tokenOrderDifference int) (weight int) {
	weight = 1
	if tokenOrderDifference < 6 {
		weight = (7 - tokenOrderDifference) * (7 - tokenOrderDifference)
	}
	return
}

// dropIrregularlyPricedResults checked that prices for products are consistent throughout the matches and drop inconsistent results
func (p *Products) dropIrregularlyPricedResults() {
	// calculate the best range
	var bestRangeStartPrice, bestRangeMaxValue, bestRangeSpread float64
	var currentRangeStartPrice, currentRangeMaxValue, currentRangeSpread float64
	var secondListingPrice float64
	var bestRangeWeightValue, currentRangeWeightValue, listingIndex, secondIndex, totalWeight int
	var currentListing, secondListing *originalListing
	var product *originalProduct
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
		for listingIndex, currentListing = range product.result.Listings {
			currentRangeStartPrice = currentListing.GetPrice(-1.0)
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
		var cuurentListing *originalListing
		if bestRangeWeightValue < totalWeight/2 {
			fmt.Println("Warning spread out pricing for product", product.ProductName, "could indicate bad matching. Discarding matches")
			for _, cuurentListing = range product.result.Listings {
				cuurentListing.match = nil
			}
			product.result.Listings = []*originalListing{}
			product.result.tokenOrderDifferences = []int{}
			continue
		}
		// drop listings the deviate too far out from the spread
		listingIndex := 0
		var allowedVariance, currentListingPrice float64
		var currentListingWeight int
		for listingIndex < len(product.result.Listings) {
			currentListing = product.result.Listings[listingIndex]
			currentListingPrice = currentListing.GetPrice(-1)
			currentListingWeight = getWeightForTokenOrderDifference(product.result.tokenOrderDifferences[listingIndex])
			allowedVariance = 1.0 + 0.05*float64(currentListingWeight)
			if currentListingPrice < bestRangeStartPrice/allowedVariance || currentListingPrice > bestRangeMaxValue*allowedVariance {
				currentListing.match = nil
				product.result.Listings = append(product.result.Listings[:listingIndex], product.result.Listings[listingIndex+1:]...)
				product.result.tokenOrderDifferences = append(product.result.tokenOrderDifferences[:listingIndex], product.result.tokenOrderDifferences[listingIndex+1:]...)
			} else {
				listingIndex++
			}
		}
	}
}

// exportResults export the results in JSON format to the given filename
func (p *Products) exportResults(filename string) {
	resultsFile, err := os.Create(filename)
	if err != nil {
		fmt.Println("Error creating file for unmatched listings:", err)
		os.Exit(1)
	}
	defer resultsFile.Close()
	jsonEncoder := json.NewEncoder(resultsFile)
	p.matchedProductCount = 0
	for _, product := range p.products {
		err = jsonEncoder.Encode(product.result)
		if err != nil {
			fmt.Println("Error exporting results to file", filename, ":", err)
			os.Exit(1)
		}
		p.matchedProductCount += len(product.result.Listings)
	}
	fmt.Println("Done writing", len(p.products), "products with", p.matchedProductCount, "matched listings to", filename)
}
