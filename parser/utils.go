package parser

import (
	"os"

	"github.com/pablor21/gonnotation/annotations"
)

// var GQLKEEP_REGEX = regexp.MustCompile(`(?s)# @gqlKeepBegin(.*?)# @gqlKeepEnd(?s)`)

// Ptr returns a pointer to the given value
func Ptr[T any](v T) *T {
	return &v
}

// DerefPtr returns the value pointed to by ptr, or defaultValue if ptr is nil
func DerefPtr[T any](ptr *T, defaultValue T) T {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}

// EnsureDir makes sure a directory exists
func EnsureDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// MatchesAnnotation checks if an annotation name matches the expected pattern with the configured prefix
// For example, with prefix "@gql" and suffix "type", it matches "@gqltype" or "@gqlType"
// If prefix is empty, only matches the suffix alone
func MatchesAnnotation(annName string, prefix string, suffixes ...string) bool {
	return annotations.MatchesAnnotation(annName, prefix, suffixes...)
}

// NormalizeAnnotationName normalizes annotation names for comparison (case-insensitive)
func NormalizeAnnotationName(name string) string {
	return annotations.NormalizeAnnotationName(name)
}

// NormalizeTagName normalizes struct tag names for comparison (case-insensitive)
func NormalizeTagName(name string) string {
	return annotations.NormalizeTagName(name)
}
