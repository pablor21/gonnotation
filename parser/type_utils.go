package parser

import "go/ast"

// TypeUtils provides utilities for working with Go AST types
type TypeUtils struct{}

func NewTypeUtils() *TypeUtils {
	return &TypeUtils{}
}

func (u *TypeUtils) GetTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return u.GetTypeName(t.X)
	case *ast.ArrayType:
		return u.GetTypeName(t.Elt)
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.MapType:
		return "map"
	default:
		return ""
	}
}

func (u *TypeUtils) IsPointer(expr ast.Expr) bool {
	_, ok := expr.(*ast.StarExpr)
	return ok
}

func (u *TypeUtils) IsSlice(expr ast.Expr) bool {
	_, ok := expr.(*ast.ArrayType)
	return ok
}

func (u *TypeUtils) IsMap(expr ast.Expr) bool {
	_, ok := expr.(*ast.MapType)
	return ok
}

func (u *TypeUtils) UnwrapPointer(expr ast.Expr) ast.Expr {
	if star, ok := expr.(*ast.StarExpr); ok {
		return star.X
	}
	return expr
}

func (u *TypeUtils) GetPackageAndType(expr ast.Expr) (pkg, typeName string) {
	baseExpr := u.UnwrapPointer(expr)
	if arr, ok := baseExpr.(*ast.ArrayType); ok {
		baseExpr = arr.Elt
		baseExpr = u.UnwrapPointer(baseExpr)
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

func (u *TypeUtils) IsBuiltinType(typeName string) bool {
	return IsGoBuiltinType(typeName)
}

func (u *TypeUtils) IsExported(name string) bool {
	if name == "" {
		return false
	}
	r := []rune(name)
	return len(r) > 0 && r[0] >= 'A' && r[0] <= 'Z'
}
