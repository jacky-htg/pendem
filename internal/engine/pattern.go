package engine

import "strings"

func matchPattern(key, pattern string) bool {
	if pattern == "*" {
		return true
	}

	// Support * wildcard
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		// *middle*
		middle := pattern[1 : len(pattern)-1]
		return strings.Contains(key, middle)
	}

	if strings.HasPrefix(pattern, "*") {
		// *suffix
		suffix := pattern[1:]
		return strings.HasSuffix(key, suffix)
	}

	if strings.HasSuffix(pattern, "*") {
		// prefix*
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(key, prefix)
	}

	return key == pattern
}
