package types

import (
	"go/ast"

	"github.com/pablor21/gonnotation/annotations"
	"github.com/pablor21/gonnotation/utils"
	"golang.org/x/tools/go/packages"
)

type TypeParam struct {
	Name       string
	Constraint *TypeInfo
	TypeRef    string // JSON-safe reference to the Constraint TypeInfo
}

// TypedElement is the common base for function parameters, return values, and struct fields
type TypedElement struct {
	Name        string
	Type        ast.Expr `json:"-"`
	IsPointer   bool
	Comment     string
	Annotations []annotations.Annotation
	Visibility  Visibility
	TypeRef     string // JSON-safe reference to the TypeInfo

	// type info for the element type
	TypeInfo *TypeInfo `json:"-"` // Prevent cycles
}

type ParameterInfo struct {
	TypedElement
}

type ReturnInfo struct {
	TypedElement
}

// parseTypedElement parses an AST type expression into a TypedElement
func parseTypedElement(typeExpr ast.Expr, name string, commentGroups []*ast.CommentGroup, genDecl *ast.GenDecl, file *ast.File, pkg *packages.Package, ctx *ProcessContext) TypedElement {
	// if the type is a pointer, set IsPointer to true and get the underlying type
	isPointer := false
	if starExpr, ok := typeExpr.(*ast.StarExpr); ok {
		isPointer = true
		typeExpr = starExpr.X
	}

	te := TypedElement{
		Name:        name,
		IsPointer:   isPointer,
		Type:        typeExpr,
		TypeInfo:    nil,
		Annotations: annotations.ParseAnnotations(commentGroups),
		Comment:     utils.ExtractCommentText(commentGroups),
		Visibility:  VisibilityPrivate, // Will be updated based on TypeInfo after it's created
	}

	// All types now use the unified TypeInfo approach
	switch t := typeExpr.(type) {
	case *ast.StructType, *ast.InterfaceType:
		// for anonymous struct/interface types, create TypeInfo with synthetic name
		syntheticName := te.Name + "Type"
		if te.Name == "" {
			syntheticName = "AnonymousType"
		}
		te.TypeInfo = GetOrCreateTypeInfo(ctx, t, syntheticName, nil, genDecl, file, pkg, nil)

	default:
		// for all other types (including arrays, slices, maps, functions), use TypeInfo
		te.TypeInfo = GetOrCreateTypeInfo(ctx, typeExpr, "", nil, genDecl, file, pkg, nil)
	}

	// Set visibility and TypeRef based on the type info or element name
	if te.TypeInfo != nil {
		te.Visibility = te.TypeInfo.Visibility
		te.TypeRef = te.TypeInfo.CannonicalName
	} else {
		// TypeInfo is nil - this might be a basic type
		if typeExpr != nil {
			if ident, ok := typeExpr.(*ast.Ident); ok && utils.IsBasicType(ident.Name) {
				// This is a basic type - set TypeRef directly and visibility to public
				te.TypeRef = ident.Name
				te.Visibility = VisibilityPublic
			} else if name != "" {
				te.Visibility = determineVisibility(name)
			} else {
				// For unnamed parameters/returns, default to public since they follow function visibility
				te.Visibility = VisibilityPublic
			}
		} else if name != "" {
			te.Visibility = determineVisibility(name)
		} else {
			// For unnamed parameters/returns, default to public since they follow function visibility
			te.Visibility = VisibilityPublic
		}
	}

	return te
}
