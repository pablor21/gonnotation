package types

import (
	"go/ast"
	"go/token"

	"github.com/pablor21/gonnotation/annotations"
)

// EnumValue represents a single enum constant value
type EnumValue struct {
	Name        string
	Visibility  Visibility
	Value       any       // The constant value (string, int, etc.)
	ValueExpr   ast.Expr  `json:"-"` // Original AST expression
	TypeInfo    *TypeInfo `json:"-"` // Type information for the value
	TypeRef     string    // JSON-safe reference to the TypeInfo
	Comment     string
	Annotations []annotations.Annotation
	Position    token.Pos `json:"-"` // Source position
}
