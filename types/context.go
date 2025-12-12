package types

import (
	"go/ast"

	"github.com/pablor21/gonnotation/config"
	"github.com/pablor21/gonnotation/logger"
	"github.com/pablor21/gonnotation/utils"
	"golang.org/x/tools/go/packages"
)

type ProcessContext struct {
	Config *config.Config
	//Generator Generator
	Logger       logger.Logger
	Types        map[string]*TypeInfo
	ConstsByType map[string][]EnumValue // Map type name to its const values for enum detection
	ModulePath   string                 // The module path of the project being scanned
}

// GetOrCreateTypeInfo retrieves a type from cache or creates it if not exists
func GetOrCreateTypeInfo(ctx *ProcessContext, expr ast.Expr, typeName string, commentGroups []*ast.CommentGroup, genDecl *ast.GenDecl, file *ast.File, pkg *packages.Package, typeSpec *ast.TypeSpec) *TypeInfo {
	if expr == nil {
		return nil
	}

	// If no context provided, just create new TypeInfo directly
	if ctx == nil {
		return NewTypeInfoFromExpr(expr, typeName, commentGroups, genDecl, file, pkg, typeSpec)
	}

	// Initialize cache if needed
	if ctx.Types == nil {
		ctx.Types = make(map[string]*TypeInfo)
	}

	// Generate canonical name for caching
	canonicalName := utils.GetCanonicalNameFromExpr(expr, typeName, pkg, file)

	// Check cache first
	if cached, exists := ctx.Types[canonicalName]; exists {
		return cached
	}

	// Create new TypeInfo if not in cache (for simple types or external references)
	ti := NewTypeInfoFromExpr(expr, typeName, commentGroups, genDecl, file, pkg, typeSpec)
	if ti != nil {
		// Ensure the canonical name matches what we use for caching
		ti.CannonicalName = canonicalName
		// Use the canonical name we computed for lookup, not the one from TypeInfo
		ctx.Types[canonicalName] = ti
		// For struct types that are being created outside the main parsing loop,
		// we should parse their fields immediately
		if ti.Kind == TypeKindStruct && ti.structType != nil {
			ti.parseFields(ctx)
		}
	}
	return ti
}
