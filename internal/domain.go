package internal

import "strings"

// ExtractRootDomainFrom extracts the root domain from an URL.
// Technically the root domain is . but in this case it strips
// any surrounding fragment, subdomain, protocol etc.
// Example: https://some.example.com will return example.com
func ExtractRootDomainFrom(url string) string {
	domain := strings.TrimSuffix(url, ".")
	s := strings.Split(domain, ".")
	if len(s) <= 2 {
		// No subdomain requested.
		return domain
	}

	return s[len(s)-2] + "." + s[len(s)-1]
}
