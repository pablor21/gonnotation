package annotations

import "strings"

// MatchesAnnotation checks if an annotation name matches the expected pattern with the configured prefix
// For example, with prefix "@gql" and suffix "type", it matches "@gqltype" or "@gqlType"
// If prefix is empty, only matches the suffix alone
func MatchesAnnotation(annName string, prefix string, suffixes ...string) bool {
	annName = strings.ToLower(annName)
	prefix = strings.ToLower(strings.TrimPrefix(prefix, "@"))

	// Check each suffix
	for _, suffix := range suffixes {
		suffix = strings.ToLower(suffix)

		// Match with prefix: e.g., "gqltype" when prefix is "gql"
		if prefix != "" {
			expected := prefix + suffix
			if annName == expected {
				return true
			}
		}

		// Also match suffix alone (without prefix) for backward compatibility
		if annName == suffix {
			return true
		}
	}

	return false
}
