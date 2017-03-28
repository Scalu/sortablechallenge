package main

import (
	"fmt"

	"github.com/Scalu/sortablechallenge/sortablechallengeutils"
)

type productToken struct {
	value    string
	products []*Product
}

func (pt *productToken) hasProduct(desiredProduct *Product) bool {
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

// BinaryTreeCompare used by BinaryTree.go
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

func (pt *ProductTokens) insert(value string) (index int) {
	pt.negativeIndexValue = value
	index, _ = pt.tokenTree.Insert(pt, -1, false)
	return
}

// GetInsertValue used by BinaryTree.go
func (pt *ProductTokens) GetInsertValue() (index int) {
	pt.tokens = append(pt.tokens, productToken{value: pt.negativeIndexValue})
	return len(pt.tokens) - 1
}

func (pt *ProductTokens) getValueString(value int) string {
	if value < 0 {
		return "--nil--"
	}
	return pt.tokens[value].value
}

// Search find the token index containing this string value
func (pt *ProductTokens) Search(stringValue string) (index int) {
	pt.negativeIndexValue = stringValue
	index, _ = pt.tokenTree.Insert(pt, -1, true)
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
func (pt *ProductTokens) AddTokens(product *Product, signature []string) (tokenList []int) {
	//build the signature
	for _, tokenString := range signature {
		tokenIndex := pt.insert(tokenString)
		if !pt.tokens[tokenIndex].hasProduct(product) {
			pt.tokens[tokenIndex].products = append(pt.tokens[tokenIndex].products, product)
			tokenList = append(tokenList, tokenIndex)
		}
	}
	return
}

// DumpTree prints out the tree to console for debugging
func (pt *ProductTokens) DumpTree() {
	pt.tokenTree.DumpTree(pt)
}

// DumpNode used by BinaryTree.go
func (pt *ProductTokens) DumpNode(value, parentValue, leftValue, rightValue, weight int) {
	fmt.Println("node", value, ":", pt.getValueString(value),
		"| parent", parentValue, ":", pt.getValueString(parentValue),
		"| left", leftValue, ":", pt.getValueString(leftValue),
		"| right", rightValue, ":", pt.getValueString(rightValue),
		"| weight", weight)
	if leftValue < 0 && weight < 0 {
		panic("invalid weight")
	}
	if rightValue < 0 && weight > 0 {
		panic("invalid weight")
	}
	if leftValue < 0 && rightValue < 0 && weight != 0 {
		panic("invalid weight!")
	}
	if leftValue > -1 && rightValue < 0 && weight > -1 {
		panic("invalid weight!")
	}
	if leftValue < 0 && rightValue > -1 && weight < 1 {
		panic("invalid weight!")
	}
}
