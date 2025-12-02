package parser

import "go/ast"

// GenericProcessor handles generic types
type GenericProcessor struct {
	resolver *TypeResolver
}

// NewGenericProcessor creates generic processor
func NewGenericProcessor(resolver *TypeResolver) *GenericProcessor {
	return &GenericProcessor{resolver: resolver}
}

// IsGenericType checks if type is generic
func (gp *GenericProcessor) IsGenericType(typeSpec *ast.TypeSpec) bool {
	return typeSpec.TypeParams != nil && typeSpec.TypeParams.NumFields() > 0
}

// GetTypeParameters extracts type parameters
func (gp *GenericProcessor) GetTypeParameters(typeSpec *ast.TypeSpec) []string {
	if typeSpec.TypeParams == nil {
		return nil
	}

	var params []string
	for _, field := range typeSpec.TypeParams.List {
		for _, name := range field.Names {
			params = append(params, name.Name)
		}
	}
	return params
}
