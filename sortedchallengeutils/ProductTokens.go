package sortedchallengeutils

import "fmt"

type productToken struct {
	value    string
	products []int
}

func (pt *productToken) hasProduct(desiredProductIndex int) bool {
	for _, productIndex := range pt.products {
		if productIndex == desiredProductIndex {
			return true
		}
	}
	return false
}

// ProductTokens This structure and it's methods handle the product tokens
// These tokens are used to matching the listings to products
type ProductTokens struct {
	tokens             []productToken
	tokenTree          BinaryTree
	negativeIndexValue string
}

func (pt *ProductTokens) binaryTreeCompare(a, b int) int {
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

func (pt *ProductTokens) insert(value string) (index int, valueAlreadyExists bool) {
	pt.negativeIndexValue = value
	return pt.tokenTree.Insert(pt, -1)
}

func (pt *ProductTokens) getInsertValue() (index int) {
	pt.tokens = append(pt.tokens, productToken{value: pt.negativeIndexValue})
	return len(pt.tokens) - 1
}

func (pt *ProductTokens) getValueString(value int) string {
	if value < 0 {
		return "--nil--"
	}
	return pt.tokens[value].value
}

func (pt *ProductTokens) Search(value string) (index int) {
	pt.negativeIndexValue = value
	return pt.tokenTree.Search(pt, -1)
}

// AddTokens Adds tokens to the tokens array
func (pt *ProductTokens) AddTokens(productIndex int, signature []string) (tokenList []int) {
	//build the signature
	for _, tokenString := range signature {
		tokenIndex, alreadyExists := pt.insert(tokenString)
		if !alreadyExists || !pt.tokens[tokenIndex].hasProduct(productIndex) {
			pt.tokens[tokenIndex].products = append(pt.tokens[tokenIndex].products, productIndex)
			tokenList = append(tokenList, tokenIndex)
		}
	}
	return
}

// DumpTree prints out the tree to console for debugging
func (pt *ProductTokens) DumpTree() {
	pt.tokenTree.DumpTree(pt)
}

func (pt *ProductTokens) dumpNode(value, parentValue, leftValue, rightValue, weight int) {
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
