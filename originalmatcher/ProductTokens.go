package originalmatcher

import (
	"github.com/Scalu/sortablechallenge/sortablechallengeutils"
)

// productToken holds the value of a token and a list of products it appears in
type productToken struct {
	value    string
	products []*originalProduct
}

// hasProduct returns true if the product is already in the token's list of products that it appears in
func (pt *productToken) hasProduct(desiredProduct *originalProduct) bool {
	for _, product := range pt.products {
		if product == desiredProduct {
			return true
		}
	}
	return false
}

// ProductTokens This structure and it's methods handle the product tokens
// These tokens are used to matching the listings to products
type ProductTokens struct {
	tokens             []productToken
	tokenTree          sortablechallengeutils.BinaryTree
	negativeIndexValue string
}

// BinaryTreeCompare used by BinaryTree.go to return the comparison indicator between two indexed values
func (pt *ProductTokens) BinaryTreeCompare(a, b int) int {
	var aValue, bValue string
	if a < 0 {
		aValue = pt.negativeIndexValue
	} else {
		aValue = pt.tokens[a].value
	}
	if b < 0 {
		bValue = pt.negativeIndexValue
	} else {
		bValue = pt.tokens[b].value
	}
	if aValue < bValue {
		return 1
	}
	if aValue > bValue {
		return -1
	}
	return 0
}

// GetInsertValue used by BinaryTree.go
func (pt *ProductTokens) GetInsertValue() (index int) {
	pt.tokens = append(pt.tokens, productToken{value: pt.negativeIndexValue})
	return len(pt.tokens) - 1
}

// Search find the token index containing this string value
func (pt *ProductTokens) Search(stringValue string) (index int) {
	pt.negativeIndexValue = stringValue
	index, _ = pt.tokenTree.Insert(-1, true)
	return
}

// GetMatchingToken returns a product token that matches a given string
func (pt *ProductTokens) getMatchingToken(value string) (matchingToken *productToken) {
	matchingIndex := pt.Search(value)
	if matchingIndex > -1 {
		return &pt.tokens[matchingIndex]
	}
	return nil
}

// AddTokens Adds tokens to the tokens array
func (pt *ProductTokens) AddTokens(product *originalProduct, signature []string) (tokenList []int) {
	//build the signature
	for _, tokenString := range signature {
		pt.negativeIndexValue = tokenString
		tokenIndex, _ := pt.tokenTree.Insert(-1, false)
		if product != nil && !pt.tokens[tokenIndex].hasProduct(product) {
			pt.tokens[tokenIndex].products = append(pt.tokens[tokenIndex].products, product)
		}
		tokenIndexFound := false
		for _, existingTokenIndex := range tokenList {
			if existingTokenIndex == tokenIndex {
				tokenIndexFound = true
			}
		}
		if !tokenIndexFound {
			tokenList = append(tokenList, tokenIndex)
		}
	}
	return
}
