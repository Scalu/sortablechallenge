package main

import (
	"encoding/json"
	"fmt"
	"math"
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
	for _, product := range p.products {
		if len(product.result.Listings) == 0 {
			continue
		}
		// get a weighted average price
		totalWeightedPrice := 0.0
		totalWeight := 0
		var currentWeight int
		var currentTokenOrderDifference int
		var currentListingPrice float64
		for listingIndex, listing := range product.result.Listings {
			currentListingPrice := listing.GetPrice(-1)
			if currentListingPrice > 0.0 {
				currentTokenOrderDifference = product.result.tokenOrderDifferences[listingIndex]
				currentWeight = getWeightForTokenOrderDifference(currentTokenOrderDifference)
				totalWeightedPrice += currentListingPrice * float64(currentWeight)
				totalWeight += currentWeight
			}
		}
		weightedAveragePrice := totalWeightedPrice / float64(totalWeight)
		totalWeightedDeviation := 0.0
		for listingIndex, listing := range product.result.Listings {
			currentListingPrice = listing.GetPrice(-1)
			if currentListingPrice > 0.0 {
				currentTokenOrderDifference = product.result.tokenOrderDifferences[listingIndex]
				currentWeight = getWeightForTokenOrderDifference(currentTokenOrderDifference)
				totalWeightedDeviation += math.Abs(weightedAveragePrice-currentListingPrice) * float64(currentWeight)
			}
		}
		weightedAverageDeviation := totalWeightedDeviation / float64(totalWeight)
		// remove all listings if deviation is very high
		var listing *Listing
		if weightedAverageDeviation >= weightedAveragePrice/3 {
			for _, listing = range product.result.Listings {
				listing.match = nil
			}
			product.result.Listings = []*Listing{}
			product.result.tokenOrderDifferences = []int{}
			continue
		}
		// drop listings the deviate too far from weighted average
		listingIndex := 0
		var allowedVariance float64
		for listingIndex < len(product.result.Listings) {
			listing = product.result.Listings[listingIndex]
			currentListingPrice = listing.GetPrice(-1)
			currentTokenOrderDifference = product.result.tokenOrderDifferences[listingIndex]
			allowedVariance = 1.0 + 6.0/(2.0+float64(currentTokenOrderDifference))
			if currentListingPrice < weightedAveragePrice/allowedVariance || currentListingPrice > weightedAveragePrice*allowedVariance {
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
