package sortablechallengeutils

// Listing defines the fields found in the listings.txt json file
type Listing struct {
	Title        string `json:"title"`
	Manufacturer string `json:"manufacturer"`
	Currency     string `json:"currency"`
	Price        string `json:"price"`
}