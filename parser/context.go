package parser

import (
	"github.com/pablor21/gonnotation/annotations"
)

// FileTypeMapping tracks which types are defined in which generated files
type FileTypeMapping struct {
	FileName string          // Generated file name
	Structs  map[string]bool // Set of struct names in this file
	Enums    map[string]bool // Set of enum names in this file
	Services map[string]bool // Set of service names in this file
}

// GenerationContext provides data and utilities for format generators
type GenerationContext struct {
	Structs            []*StructInfo    // Filtered structs (annotated for this format generator)
	Interfaces         []*InterfaceInfo // Filtered interfaces (annotated for this format generator)
	Enums              []*EnumInfo      // Filtered enums (annotated for this format generator)
	Functions          []*FunctionInfo  // Filtered functions (annotated for this format generator)
	AllStructs         []*StructInfo    // All parsed structs (for dependency resolution)
	AllInterfaces      []*InterfaceInfo // All parsed interfaces (for dependency resolution)
	AllEnums           []*EnumInfo      // All parsed enums (for dependency resolution)
	AllFunctions       []*FunctionInfo  // All parsed functions (for reference)
	EmptyTypes         map[string]bool  // Types with no exported fields (should be excluded from generation)
	Parser             *Parser
	CoreConfig         *CoreConfig
	PluginConfig       any
	AutoGenerateConfig *AutoGenerateConfig // Merged root + format generator auto-generate config
	Logger             Logger              // Logger for generation messages

	// File mapping for multi-file generation
	FileTypeMappings []*FileTypeMapping // Track which types are in which files

	// Package and file-level annotations
	PackageAnnotations map[string][]annotations.Annotation // Package path -> annotations
	FileAnnotations    map[string][]annotations.Annotation // File path -> annotations

	// Shared utilities
	TypeResolver     *TypeResolver
	GenericProcessor *GenericProcessor
}

// NewGenerationContext creates context with utilities (backward-compatible without interfaces)
func NewGenerationContext(parser *Parser, structs []*StructInfo, enums []*EnumInfo, functions []*FunctionInfo, allStructs []*StructInfo, allEnums []*EnumInfo, allFunctions []*FunctionInfo, coreConfig *CoreConfig, pluginConfig any, autoGenConfig *AutoGenerateConfig) *GenerationContext {
	ctx := &GenerationContext{
		Structs:            structs,
		Interfaces:         nil,
		Enums:              enums,
		Functions:          functions,
		AllStructs:         allStructs,
		AllInterfaces:      nil,
		AllEnums:           allEnums,
		AllFunctions:       allFunctions,
		Parser:             parser,
		CoreConfig:         coreConfig,
		PluginConfig:       pluginConfig,
		AutoGenerateConfig: autoGenConfig,
		Logger:             NewDefaultLogger(),
		PackageAnnotations: make(map[string][]annotations.Annotation),
		FileAnnotations:    make(map[string][]annotations.Annotation),
	}

	ctx.TypeResolver = NewTypeResolver(parser)
	ctx.GenericProcessor = NewGenericProcessor(ctx.TypeResolver)
	// Note: Field processing and naming strategies are format-specific and should be handled by plugins

	// Extract file-level and package-level annotations
	if parser != nil {
		fileAnns, pkgAnns := parser.ExtractFileAnnotations()
		ctx.FileAnnotations = fileAnns
		ctx.PackageAnnotations = pkgAnns
	}

	return ctx
}

// NewGenerationContextWithInterfaces creates context including interfaces
func NewGenerationContextWithInterfaces(parser *Parser, structs []*StructInfo, interfaces []*InterfaceInfo, enums []*EnumInfo, functions []*FunctionInfo, allStructs []*StructInfo, allInterfaces []*InterfaceInfo, allEnums []*EnumInfo, allFunctions []*FunctionInfo, coreConfig *CoreConfig, pluginConfig any, autoGenConfig *AutoGenerateConfig, emptyTypes map[string]bool) *GenerationContext {
	ctx := &GenerationContext{
		Structs:            structs,
		Interfaces:         interfaces,
		Enums:              enums,
		Functions:          functions,
		AllStructs:         allStructs,
		AllInterfaces:      allInterfaces,
		AllEnums:           allEnums,
		AllFunctions:       allFunctions,
		EmptyTypes:         emptyTypes,
		Parser:             parser,
		CoreConfig:         coreConfig,
		PluginConfig:       pluginConfig,
		AutoGenerateConfig: autoGenConfig,
		Logger:             NewDefaultLogger(),
		PackageAnnotations: make(map[string][]annotations.Annotation),
		FileAnnotations:    make(map[string][]annotations.Annotation),
	}

	ctx.TypeResolver = NewTypeResolver(parser)
	ctx.GenericProcessor = NewGenericProcessor(ctx.TypeResolver)
	// Note: Field processing and naming strategies are format-specific and should be handled by plugins

	// Extract file-level and package-level annotations
	if parser != nil {
		fileAnns, pkgAnns := parser.ExtractFileAnnotations()
		ctx.FileAnnotations = fileAnns
		ctx.PackageAnnotations = pkgAnns
	}

	return ctx
}

// ImplementsGoInterface reports if struct s implements the given Go interface itf.
// This uses the parser's method-set check against ctx.AllFunctions.
func (ctx *GenerationContext) ImplementsGoInterface(s *StructInfo, itf *InterfaceInfo) bool {
	if ctx == nil || ctx.Parser == nil || s == nil || itf == nil {
		return false
	}
	return ctx.Parser.ImplementsGoInterface(s, itf, ctx.AllFunctions)
}

// ImplementsAnnotatedInterface reports if struct s "implements" the annotated-struct interface ifaceStruct
// by having at least the same exported Go fields.
func (ctx *GenerationContext) ImplementsAnnotatedInterface(s *StructInfo, ifaceStruct *StructInfo) bool {
	if ctx == nil || ctx.Parser == nil || s == nil || ifaceStruct == nil {
		return false
	}
	return ctx.Parser.ImplementsAnnotatedInterface(s, ifaceStruct)
}

// InferImplementedInterfaceNames returns interface names that struct implements.
// This is now a generic helper - plugins should implement their own specific interface inference logic.
func (ctx *GenerationContext) InferImplementedInterfaceNames(s *StructInfo) []string {
	// Interface inference is now plugin-specific responsibility
	// This method is kept for backward compatibility but plugins should implement their own logic
	return nil
}

// AddFileTypeMapping adds a file type mapping to track which types are in which files
func (ctx *GenerationContext) AddFileTypeMapping(fileName string) *FileTypeMapping {
	mapping := &FileTypeMapping{
		FileName: fileName,
		Structs:  make(map[string]bool),
		Enums:    make(map[string]bool),
		Services: make(map[string]bool),
	}
	ctx.FileTypeMappings = append(ctx.FileTypeMappings, mapping)
	return mapping
}

// GetFileTypeMapping gets the mapping for a specific file
func (ctx *GenerationContext) GetFileTypeMapping(fileName string) *FileTypeMapping {
	for _, mapping := range ctx.FileTypeMappings {
		if mapping.FileName == fileName {
			return mapping
		}
	}
	return nil
}

// FindTypeFile finds which file contains a specific type
func (ctx *GenerationContext) FindTypeFile(typeName string) string {
	for _, mapping := range ctx.FileTypeMappings {
		if mapping.Structs[typeName] || mapping.Enums[typeName] || mapping.Services[typeName] {
			return mapping.FileName
		}
	}
	return ""
}

// GetRequiredImports returns the list of files that need to be imported for types used in the given file
func (ctx *GenerationContext) GetRequiredImports(currentFile string, usedTypes []string) []string {
	var imports []string
	importSet := make(map[string]bool)

	for _, typeName := range usedTypes {
		typeFile := ctx.FindTypeFile(typeName)
		if typeFile != "" && typeFile != currentFile && !importSet[typeFile] {
			imports = append(imports, typeFile)
			importSet[typeFile] = true
		}
	}

	return imports
}
