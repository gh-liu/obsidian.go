package completion

import "strings"

func fileMatchScore(prefixLower, displayLower, pathLower string, aliases []string) int {
	if prefixLower == "" {
		return 1
	}
	displayMatch := matchLevel(prefixLower, displayLower)
	pathMatch := matchLevel(prefixLower, pathLower)
	best := max(displayMatch, pathMatch)
	if best == 2 {
		return 3
	}
	if best == 1 {
		return 2
	}
	for _, alias := range aliases {
		if matchLevel(prefixLower, strings.ToLower(alias)) > 0 {
			return 1
		}
	}
	return 0
}

func headingMatchScore(prefixLower, headingLower string) int {
	if prefixLower == "" {
		return 1
	}
	return matchLevel(prefixLower, headingLower)
}

func blockMatchScore(prefixLower, blockIDLower string) int {
	if prefixLower == "" {
		return 1
	}
	return matchLevel(prefixLower, blockIDLower)
}

func matchLevel(prefixLower, candidateLower string) int {
	if strings.HasPrefix(candidateLower, prefixLower) {
		return 2
	}
	if strings.Contains(candidateLower, prefixLower) {
		return 1
	}
	return 0
}
