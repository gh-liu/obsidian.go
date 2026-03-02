package completion

import "strings"

func fileMatchScore(prefixLower, display, path string, aliases []string) int {
	if prefixLower == "" {
		return 1
	}
	if hasPrefixOrContains(prefixLower, display) == 2 || hasPrefixOrContains(prefixLower, path) == 2 {
		return 3
	}
	if hasPrefixOrContains(prefixLower, display) == 1 || hasPrefixOrContains(prefixLower, path) == 1 {
		return 2
	}
	for _, alias := range aliases {
		if hasPrefixOrContains(prefixLower, alias) > 0 {
			return 1
		}
	}
	return 0
}

func headingMatchScore(prefixLower, heading string) int {
	if prefixLower == "" {
		return 1
	}
	return hasPrefixOrContains(prefixLower, heading)
}

func hasPrefixOrContains(prefixLower, candidate string) int {
	candidateLower := strings.ToLower(candidate)
	if strings.HasPrefix(candidateLower, prefixLower) {
		return 2
	}
	if strings.Contains(candidateLower, prefixLower) {
		return 1
	}
	return 0
}
