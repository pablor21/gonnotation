package parser

import (
	"go/ast"
	"strings"
)

// TypeResolver provides type resolution utilities
type TypeResolver struct {
	parser      *Parser
	currentFile *ast.File         // Track current file being processed for import resolution
	importPaths map[string]string // Map of package name -> import path for current file
}

// NewTypeResolver creates a type resolver
func NewTypeResolver(parser *Parser) *TypeResolver {
	return &TypeResolver{
		parser:      parser,
		importPaths: make(map[string]string),
	}
}

// SetCurrentFile sets the current file context for import resolution
func (tr *TypeResolver) SetCurrentFile(file *ast.File) {
	tr.currentFile = file
	tr.importPaths = make(map[string]string)

	if file != nil {
		// Build import map for this file
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)

			// Get package name (either from alias or last component of path)
			var pkgName string
			if imp.Name != nil {
				pkgName = imp.Name.Name
			} else {
				// Use last component of import path as package name
				parts := strings.Split(importPath, "/")
				pkgName = parts[len(parts)-1]
				// Handle versioned paths like /v2, /v3
				if len(parts) > 1 && strings.HasPrefix(pkgName, "v") && len(pkgName) <= 3 {
					pkgName = parts[len(parts)-2]
				}
			}

			tr.importPaths[pkgName] = importPath
		}
	}
}

// GetQualifiedTypeName returns the fully qualified type name including package path
// For example: uuid.UUID -> github.com/google/uuid.UUID
func (tr *TypeResolver) GetQualifiedTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return tr.GetQualifiedTypeName(t.X)
	case *ast.ArrayType:
		return tr.GetQualifiedTypeName(t.Elt)
	case *ast.SelectorExpr:
		// Get package identifier
		if pkgIdent, ok := t.X.(*ast.Ident); ok {
			pkgName := pkgIdent.Name
			// Look up import path
			if importPath, exists := tr.importPaths[pkgName]; exists {
				return importPath + "." + t.Sel.Name
			}
			// Fallback to package.Type format
			return pkgName + "." + t.Sel.Name
		}
		return t.Sel.Name
	case *ast.MapType:
		return "map"
	default:
		return ""
	}
}

// GetTypeName extracts type name from expression
func (tr *TypeResolver) GetTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return tr.GetTypeName(t.X)
	case *ast.ArrayType:
		return tr.GetTypeName(t.Elt)
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.MapType:
		return "map"
	default:
		return ""
	}
}

// IsPointer checks if expression is pointer
func (tr *TypeResolver) IsPointer(expr ast.Expr) bool {
	_, ok := expr.(*ast.StarExpr)
	return ok
}

// IsSlice checks if expression is slice
func (tr *TypeResolver) IsSlice(expr ast.Expr) bool {
	_, ok := expr.(*ast.ArrayType)
	return ok
}

// IsMap checks if expression is map
func (tr *TypeResolver) IsMap(expr ast.Expr) bool {
	_, ok := expr.(*ast.MapType)
	return ok
}

// UnwrapPointer removes pointer indirection
func (tr *TypeResolver) UnwrapPointer(expr ast.Expr) ast.Expr {
	if star, ok := expr.(*ast.StarExpr); ok {
		return star.X
	}
	return expr
}

// IsBuiltinType checks if type is Go builtin
func (tr *TypeResolver) IsBuiltinType(name string) bool {
	return IsGoBuiltinType(name)
}

// GetPackageAndType extracts package name and type name from an expression
func (tr *TypeResolver) GetPackageAndType(expr ast.Expr) (pkg, typeName string) {
	baseExpr := tr.UnwrapPointer(expr)
	if arr, ok := baseExpr.(*ast.ArrayType); ok {
		baseExpr = arr.Elt
		baseExpr = tr.UnwrapPointer(baseExpr)
	}

	switch t := baseExpr.(type) {
	case *ast.SelectorExpr:
		if pkgIdent, ok := t.X.(*ast.Ident); ok {
			return pkgIdent.Name, t.Sel.Name
		}
	case *ast.Ident:
		return "", t.Name
	}

	return "", ""
}
