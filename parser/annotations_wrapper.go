package parser

import (
	"go/ast"

	"github.com/pablor21/gonnotation/annotations"
)

// ParseAnnotations extracts annotations from comment groups
// This is a wrapper that maintains backward compatibility
func ParseAnnotations(comments []*ast.CommentGroup) []annotations.Annotation {
	return annotations.ParseAnnotations(comments)
}

// ExtractFileLevelNamespace extracts the @namespace annotation from file-level comments
// This is a wrapper that maintains backward compatibility
func ExtractFileLevelNamespace(file *ast.File) string {
	return annotations.ExtractFileLevelNamespace(file)
}

// ExtractTypeNamespace extracts the @namespace annotation from type-level annotations
// This is a wrapper that maintains backward compatibility
func ExtractTypeNamespace(anns []annotations.Annotation) string {
	return annotations.ExtractTypeNamespace(anns)
}
