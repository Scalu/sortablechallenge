package sortablechallengeutils

// Product defines the fields found in the products.txt json file
type Product struct {
	ProductName            string `json:"product_name"`
	Manufacturer           string `json:"manufacturer"`
	Model                  string `json:"model"`
	Family                 string `json:"family"`
	AnnouncedDate          string `json:"announced_date"`
}

