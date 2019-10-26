package internal

import "strings"

func ExtractRootDomainFrom(url string) string {
	domain := strings.TrimSuffix(url, ".")
	s := strings.Split(domain, ".")
	if len(s) <= 2 {
		// No subdomain requested.
		return domain
	}

	return s[len(s)-2] + "." + s[len(s)-1]
}
