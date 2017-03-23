package sortedchallengeutils

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Product defines the fields found in the products.txt json file
type Product struct {
	ProductName   string `json:"product_name"`
	Manufacturer  string `json:"manufacturer"`
	Model         string `json:"model"`
	Family        string `json:"family"`
	AnnouncedDate string `json:"announced_date"`
	tokenList     []int
}

// Products implements common interface for loading json data
type Products struct {
	products []Product
}

func (p *Products) fileName() string {
	return "products.txt"
}

func (p *Products) decode(decoder *json.Decoder) (err error) {
	product := &Product{}
	err = decoder.Decode(&product)
	if err == nil {
		p.products = append(p.products, *product)
	}
	return
}

func generateTokensFromString(value string) (tokens []string) {
	tokenStart := 0
	value = strings.ToLower(value)
	for i := 0; i < len(value); i++ {
		if !(value[i] >= 'a' && value[i] <= 'z' || value[i] >= '0' && value[i] <= '9') {
			if i > tokenStart {
				tokens = append(tokens, value[tokenStart:i])
			}
			tokenStart = i + 1
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
	for productIndex, product := range p.products {
		tokenArray := []string{}
		tokenArray = append(tokenArray, generateTokensFromString(product.Manufacturer)...)
		tokenArray = append(tokenArray, generateTokensFromString(product.Family)...)
		tokenArray = append(tokenArray, generateTokensFromString(product.Model)...)
		product.tokenList = productTokens.AddTokens(productIndex, tokenArray)
	}
	fmt.Println("Product tokens generated. ", len(p.products), " products, ", len(productTokens.tokens), " tokens")
	return
}

// GetProductCount returns the number of products in the array
func (p *Products) GetProductCount() int {
	return len(p.products)
}
