package originalmatcherplusconcurrency

import "strings"

// breaks a string up into 'tokens' for matching. Used by Products.go and Listings.go
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
