package sortablechallengeutils

// Result contains matching results to be exported
type Result struct {
	ProductName           string     `json:"product_name"`
	Listings              []*Listing `json:"listings"`
	tokenOrderDifferences []int
}
