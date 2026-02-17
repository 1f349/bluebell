package validation

import "strings"

// IsValidHost ensures a host string is valid.
//
// - Each rune must match 0-9, a-z, "-" or ".".
// - Hosts are separated by "." into segments and each segment must not be empty.
// - Naturally this also ensures the host does not start or end with ".".
// - Host segments must not start or end with "-".
func IsValidHost(domain string) bool {
	if !containsOnly(domain, isDomainRune) {
		return false
	}

	segments := strings.SplitSeq(domain, ".")
	for segment := range segments {
		if segment == "" || strings.HasPrefix(segment, "-") || strings.HasSuffix(segment, "-") {
			return false
		}
	}
	return true
}

func IsValidSite(site string) bool {
	if len(site) < 1 || site[0] == '-' {
		return false
	}
	return containsOnly(site, func(r rune) bool {
		return isDomainRune(r) || r == '_'
	})
}

func IsValidBranch(branch string) bool {
	if len(branch) < 1 {
		return false
	}
	switch branch[0] {
	case '-', '/':
		return false
	}
	if branch[len(branch)-1] == '/' {
		return false
	}
	return containsOnly(branch, func(r rune) bool {
		return isDomainRune(r) || (r >= 'A' && r <= 'Z') || r == '_' || r == '/'
	})
}

func isDomainRune(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || r == '-' || r == '.'
}

func containsOnly(s string, f func(r rune) bool) bool {
	for _, r := range []rune(s) {
		if !f(r) {
			return false
		}
	}
	return true
}
