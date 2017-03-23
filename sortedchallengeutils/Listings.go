package sortedchallengeutils

import (
	"encoding/json"
)

// Listing defines the fields found in the listings.txt json file
type Listing struct {
	Title        string `json:"title"`
	Manufacturer string `json:"manufacturer"`
	Currency     string `json:"currency"`
	Price        string `json:"price"`
}

// Listings struct to hold the listing data
// implementes JSONDecoder
type Listings struct {
	listings []Listing
}

func (l *Listings) fileName() string {
	return "listings.txt"
}

func (l *Listings) decode(decoder *json.Decoder) (err error) {
	listing := Listing{}
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
