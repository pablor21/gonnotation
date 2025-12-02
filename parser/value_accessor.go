package parser

import (
	"fmt"
	"go/ast"
	"strconv"
)

// ValueAccessor provides a common interface for accessing values from both
// ast.BasicLit and ast.Ident nodes. This eliminates code duplication across
// multiple functions that need to extract values from these node types.
type ValueAccessor interface {
	// GetValue returns the value as a string and a boolean indicating success
	GetValue() (string, bool)
}

// BasicLitAccessor wraps ast.BasicLit to implement ValueAccessor
type BasicLitAccessor struct {
	Lit *ast.BasicLit
}

func (b BasicLitAccessor) GetValue() (string, bool) {
	if b.Lit == nil {
		return "", false
	}
	return b.Lit.Value, true
}

// IdentAccessor wraps ast.Ident to implement ValueAccessor
type IdentAccessor struct {
	Ident *ast.Ident
}

func (i IdentAccessor) GetValue() (string, bool) {
	if i.Ident == nil {
		return "", false
	}
	return i.Ident.Name, true
}

// GetStringValue extracts a string value from a ValueAccessor, handling quoted strings.
// Returns the unquoted string and true on success, or empty string and false on failure.
func GetStringValue(accessor ValueAccessor) (string, bool) {
	if accessor == nil {
		return "", false
	}

	val, ok := accessor.GetValue()
	if !ok {
		return "", false
	}

	// Handle quoted strings
	if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		unquoted, err := strconv.Unquote(val)
		if err == nil {
			return unquoted, true
		}
	}

	return val, true
}

// GetBoolValue extracts a boolean value from a ValueAccessor.
// Returns the boolean value and true on success, or false and false on failure.
func GetBoolValue(accessor ValueAccessor) (bool, bool) {
	if accessor == nil {
		return false, false
	}

	val, ok := accessor.GetValue()
	if !ok {
		return false, false
	}

	switch val {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
}

// GetIntValue extracts an integer value from a ValueAccessor.
// Returns the integer value and true on success, or 0 and false on failure.
func GetIntValue(accessor ValueAccessor) (int, bool) {
	if accessor == nil {
		return 0, false
	}

	val, ok := accessor.GetValue()
	if !ok {
		return 0, false
	}

	i, err := strconv.Atoi(val)
	if err != nil {
		return 0, false
	}

	return i, true
}

// CreateAccessor creates a ValueAccessor from an ast.Expr.
// Returns nil if the expression is not a BasicLit or Ident.
func CreateAccessor(expr ast.Expr) ValueAccessor {
	switch v := expr.(type) {
	case *ast.BasicLit:
		return BasicLitAccessor{Lit: v}
	case *ast.Ident:
		return IdentAccessor{Ident: v}
	default:
		return nil
	}
}

// MustGetStringValue is like GetStringValue but panics on failure.
// Use this only when you're certain the value exists and is a string.
func MustGetStringValue(accessor ValueAccessor) string {
	val, ok := GetStringValue(accessor)
	if !ok {
		panic(fmt.Sprintf("failed to get string value from accessor: %T", accessor))
	}
	return val
}
