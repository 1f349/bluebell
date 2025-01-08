package validation

func IsValidSite(site string) bool {
	if len(site) < 1 || site[0] == '-' {
		return false
	}
	switch site[0] {
	case '-':
		return false
	}
	return containsOnly(site, func(r rune) bool {
		return isAlphanumericOrDash(r) || r == '.'
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
		return isAlphanumericOrDash(r) || r == '/' || r == '.'
	})
}

func isAlphanumericOrDash(r rune) bool {
	switch {
	case r >= '0' && r <= '9':
		return true
	case r >= 'a' && r <= 'z':
		return true
	case r >= 'A' && r <= 'Z':
		return true
	case r == '-', r == '_':
		return true
	default:
		return false
	}
}

func containsOnly(s string, f func(r rune) bool) bool {
	for _, r := range []rune(s) {
		if !f(r) {
			return false
		}
	}
	return true
}
