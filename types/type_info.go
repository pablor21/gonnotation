package types

import (
	"go/ast"
	"strings"

	"github.com/pablor21/gonnotation/annotations"
	"github.com/pablor21/gonnotation/config"
	"github.com/pablor21/gonnotation/utils"
	"golang.org/x/tools/go/packages"
)

type TypeKind string

const (
	TypeKindStruct    TypeKind = "struct"
	TypeKindInterface TypeKind = "interface"
	TypeKindFunction  TypeKind = "function"
	TypeKindEnum      TypeKind = "enum"
	TypeKindBasic     TypeKind = "basic"
	TypeKindAlias     TypeKind = "alias"
	TypeKindArray     TypeKind = "array"
	TypeKindSlice     TypeKind = "slice"
	TypeKindMap       TypeKind = "map"
)

type Visibility string

const (
	VisibilityPublic    Visibility = "public"
	VisibilityProtected Visibility = "protected"
	VisibilityPrivate   Visibility = "private"
)

type TypeInfo struct {
	Package                *packages.Package
	Kind                   TypeKind
	Visibility             Visibility
	CannonicalName         string // PackagePath +  "." +  Name
	Name                   string
	File                   *ast.File     `json:"-"`
	TypeSpec               *ast.TypeSpec `json:"-"`
	GenDecl                *ast.GenDecl  `json:"-"`
	Expr                   ast.Expr      `json:"-"` // Original expression that created this type
	PkgName                string
	PkgPath                string
	Comment                string
	IsGeneric              bool
	IsAlias                bool
	AliasTarget            *TypeInfo `json:"-"`
	AliasTargetRef         string    // JSON-safe reference to AliasTarget
	IsGenericInstantiation bool
	BaseGenericType        *TypeInfo   `json:"-"`
	BaseGenericTypeRef     string      // JSON-safe reference to BaseGenericType
	TypeArguments          []*TypeInfo `json:"-"`
	TypeArgumentRefs       []string    // JSON-safe references to TypeArguments
	TypeParams             []TypeParam
	// Container type information
	ElementType    *TypeInfo `json:"-"` // For arrays, slices, maps (value type)
	ElementTypeRef string    // JSON-safe reference to ElementType
	KeyType        *TypeInfo `json:"-"` // For maps only
	KeyTypeRef     string    // JSON-safe reference to KeyType
	// Function type information
	FunctionSig *FunctionInfo // For function types (reuse existing structure)
	//
	Annotations []annotations.Annotation
	Fields      []FieldInfo
	Methods     []FunctionInfo
	EnumValues  []EnumValue // For enum types

	// Internal field to store struct AST for deferred parsing
	structType    *ast.StructType    `json:"-"` // for deferred field parsing
	interfaceType *ast.InterfaceType `json:"-"` // for deferred method parsing
	typeParamList *ast.FieldList     `json:"-"` // for deferred constraint parsing
}

func NewTypeInfoFromAst(spec ast.Spec, genDecl *ast.GenDecl, file *ast.File, pkg *packages.Package) *TypeInfo {
	switch ts := spec.(type) {
	case *ast.TypeSpec:
		return NewTypeInfoFromExpr(ts.Type, ts.Name.Name, []*ast.CommentGroup{ts.Doc, genDecl.Doc}, genDecl, file, pkg, ts)
	}
	return nil
}

// NewTypeInfoFromExpr creates TypeInfo from an ast.Expr with optional context
// determineVisibility returns the visibility of a named element based on Go naming conventions
func determineVisibility(name string) Visibility {
	if name == "" {
		return VisibilityPrivate
	}
	// Basic types are always public
	if utils.IsBasicType(name) {
		return VisibilityPublic
	}
	// In Go, names starting with uppercase are public, lowercase are private
	if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
		return VisibilityPublic
	}
	return VisibilityPrivate
}

func NewTypeInfoFromExpr(expr ast.Expr, typeName string, commentGroups []*ast.CommentGroup, genDecl *ast.GenDecl, file *ast.File, pkg *packages.Package, typeSpec *ast.TypeSpec) *TypeInfo {
	if expr == nil {
		return nil
	}

	// Handle pointer types by unwrapping them
	if starExpr, ok := expr.(*ast.StarExpr); ok {
		return NewTypeInfoFromExpr(starExpr.X, "", commentGroups, genDecl, file, pkg, typeSpec)
	}

	// Parse ann and comments if provided
	var ann []annotations.Annotation
	var comments string
	if commentGroups != nil {
		ann = annotations.ParseAnnotations(commentGroups)
		comments = utils.ExtractCommentText(commentGroups)
	}

	// Determine canonical name
	canonicalName := typeName
	if pkg != nil && typeName != "" {
		if !utils.IsBasicType(typeName) {
			canonicalName = pkg.PkgPath + "." + typeName
		}
	}

	// Create base TypeInfo
	ti := &TypeInfo{
		CannonicalName: canonicalName,
		File:           file,
		Name:           typeName,
		Comment:        comments,
		TypeSpec:       typeSpec,
		GenDecl:        genDecl,
		Expr:           expr, // Store the original expression
		Visibility:     determineVisibility(typeName),
		Fields:         []FieldInfo{},
		Methods:        []FunctionInfo{},
		Annotations:    ann,
		TypeParams:     []TypeParam{},
		EnumValues:     []EnumValue{},
	}

	// Only set package info for local types (not for external references)
	// External types will have their package info set in their specific handling below

	// Check if this is a type alias
	if typeSpec != nil && typeSpec.Assign.IsValid() {
		ti.Kind = TypeKindAlias
		ti.IsAlias = true
		// This is a local alias definition, assign the current package
		ti.Package = pkg
		if pkg != nil {
			ti.PkgName = pkg.Name
			ti.PkgPath = pkg.PkgPath
		}
		// The alias target will be parsed later to avoid recursion issues
		return ti
	}

	// Set generic info if available
	if typeSpec != nil && typeSpec.TypeParams != nil {
		ti.IsGeneric = len(typeSpec.TypeParams.List) > 0
		// Store for deferred parsing after type is cached
		ti.typeParamList = typeSpec.TypeParams
		// Create placeholder TypeParams with names only
		for _, param := range typeSpec.TypeParams.List {
			for _, name := range param.Names {
				typeParam := TypeParam{
					Name:       name.Name,
					Constraint: nil, // Will be parsed later
				}
				ti.TypeParams = append(ti.TypeParams, typeParam)
			}
		}
	}

	// Handle different expression types
	switch t := expr.(type) {
	case *ast.StructType:
		ti.Kind = TypeKindStruct
		// Don't parse fields immediately - this will be done later to handle self-references
		ti.structType = t
		// This is a local type definition, assign the current package
		ti.Package = pkg
		if pkg != nil {
			ti.PkgName = pkg.Name
			ti.PkgPath = pkg.PkgPath
		}

	case *ast.InterfaceType:
		ti.Kind = TypeKindInterface
		// Don't parse methods immediately - this will be done later to handle self-references
		ti.interfaceType = t
		// This is a local type definition, assign the current package
		ti.Package = pkg
		if pkg != nil {
			ti.PkgName = pkg.Name
			ti.PkgPath = pkg.PkgPath
		}

	case *ast.FuncType:
		ti.Kind = TypeKindFunction
		// This is a local type definition, assign the current package
		ti.Package = pkg
		if pkg != nil {
			ti.PkgName = pkg.Name
			ti.PkgPath = pkg.PkgPath
		}

	case *ast.Ident:
		// Named type reference or underlying type for type definitions
		if typeName != "" {
			// This is a type definition: type MyType int
			// Use the provided typeName, not the underlying type name
			ti.Kind = TypeKindStruct // Will be corrected to enum if constants are found
			ti.Name = typeName
			if pkg != nil {
				ti.CannonicalName = pkg.PkgPath + "." + typeName
			} else {
				ti.CannonicalName = typeName
			}
			// This is a local type definition, assign the current package
			ti.Package = pkg
			if pkg != nil {
				ti.PkgName = pkg.Name
				ti.PkgPath = pkg.PkgPath
			}
		} else if utils.IsBasicType(t.Name) {
			// Don't create TypeInfo for basic types - they will be handled via TypeRef only
			return nil
		} else {
			// This is a reference to another custom type
			ti.Kind = TypeKindStruct
			ti.Name = t.Name
			if pkg != nil {
				ti.CannonicalName = pkg.PkgPath + "." + t.Name
			} else {
				ti.CannonicalName = t.Name
			}
			// This is a local type reference, assign the current package
			ti.Package = pkg
			if pkg != nil {
				ti.PkgName = pkg.Name
				ti.PkgPath = pkg.PkgPath
			}
		}

	case *ast.SelectorExpr:
		// Qualified type reference (e.g., time.Time)
		ti.Kind = TypeKindStruct
		if ident, ok := t.X.(*ast.Ident); ok {
			if typeName == "" {
				ti.Name = t.Sel.Name
			}
			ti.PkgName = ident.Name
			// Resolve the actual package path using the current package's imports
			actualPkgPath := utils.ResolvePackagePathFromImports(ident.Name, pkg, file)
			ti.PkgPath = actualPkgPath
			ti.CannonicalName = actualPkgPath + "." + t.Sel.Name
			// Create a minimal Package structure for external types
			ti.Package = &packages.Package{
				ID:      actualPkgPath,
				Name:    ident.Name,
				PkgPath: actualPkgPath,
			}
		}

	case *ast.ArrayType:
		// Array or slice type - create proper container type
		if t.Len != nil {
			ti.Kind = TypeKindArray
		} else {
			ti.Kind = TypeKindSlice
		}
		// This is a local type definition, assign the current package
		ti.Package = pkg
		if pkg != nil {
			ti.PkgName = pkg.Name
			ti.PkgPath = pkg.PkgPath
		}
		// Store element type for deferred parsing
		ti.structType = &ast.StructType{} // Placeholder to trigger element type parsing

	case *ast.MapType:
		// Map type - create proper container type
		ti.Kind = TypeKindMap
		// This is a local type definition, assign the current package
		ti.Package = pkg
		if pkg != nil {
			ti.PkgName = pkg.Name
			ti.PkgPath = pkg.PkgPath
		}
		// Store key and element types for deferred parsing
		ti.structType = &ast.StructType{} // Placeholder to trigger element type parsing

	case *ast.IndexExpr:
		// Generic type instantiation with single type argument
		ti.Kind = TypeKindStruct // Will be updated to correct kind in parseGenericInstantiation
		ti.IsGenericInstantiation = true
		// This is a local type definition, assign the current package
		ti.Package = pkg
		if pkg != nil {
			ti.PkgName = pkg.Name
			ti.PkgPath = pkg.PkgPath
		}
		// Store base type and type argument for deferred parsing
		ti.structType = &ast.StructType{} // Placeholder to trigger field inheritance

	case *ast.IndexListExpr:
		// Generic type instantiation with multiple type arguments
		ti.Kind = TypeKindStruct // Will be updated to correct kind in parseGenericInstantiation
		ti.IsGenericInstantiation = true
		// This is a local type definition, assign the current package
		ti.Package = pkg
		if pkg != nil {
			ti.PkgName = pkg.Name
			ti.PkgPath = pkg.PkgPath
		}
		// Store base type and type arguments for deferred parsing
		ti.structType = &ast.StructType{} // Placeholder to trigger field inheritance

	default:
		// Unknown type, create a basic TypeInfo
		ti.Kind = TypeKindStruct
		// This is likely a local type, assign the current package
		ti.Package = pkg
		if pkg != nil {
			ti.PkgName = pkg.Name
			ti.PkgPath = pkg.PkgPath
		}
	}

	return ti
}

// parseMethods extracts methods associated with a type from the file
func (ti *TypeInfo) parseMethods(file *ast.File, typeName string) {
	// This method is called from parseFields, so we don't have direct access to ParsingContext
	// Methods filtering will be handled at a higher level when this is called
	for _, fdecl := range file.Decls {
		funcDecl, ok := fdecl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}
		// check if the receiver type matches the type name
		recvTypeExpr := funcDecl.Recv.List[0].Type
		var recvTypeName string
		switch rt := recvTypeExpr.(type) {
		case *ast.Ident:
			recvTypeName = rt.Name
		case *ast.StarExpr:
			if ident, ok := rt.X.(*ast.Ident); ok {
				recvTypeName = ident.Name
			}
		}
		if recvTypeName == typeName {
			fi := FunctionInfo{
				Name:        funcDecl.Name.Name,
				FuncDecl:    funcDecl,
				File:        file,
				Comment:     utils.ExtractCommentText([]*ast.CommentGroup{funcDecl.Doc}),
				Annotations: annotations.ParseAnnotations([]*ast.CommentGroup{funcDecl.Doc}),
				Parms:       []ParameterInfo{},
				Returns:     []ReturnInfo{},
				Visibility:  determineVisibility(funcDecl.Name.Name),
			}
			// parse parameters
			if funcDecl.Type.Params != nil {
				for _, param := range funcDecl.Type.Params.List {
					for _, paramName := range param.Names {
						paramTypedElem := parseTypedElement(param.Type, paramName.Name, nil, ti.GenDecl, file, ti.Package, nil)
						pi := ParameterInfo{
							TypedElement: paramTypedElem,
						}
						fi.Parms = append(fi.Parms, pi)
					}
				}
			}
			// parse return values
			if funcDecl.Type.Results != nil {
				for _, result := range funcDecl.Type.Results.List {
					if len(result.Names) > 0 {
						// named return values
						for _, returnName := range result.Names {
							returnTypedElem := parseTypedElement(result.Type, returnName.Name, nil, ti.GenDecl, file, ti.Package, nil)
							ri := ReturnInfo{
								TypedElement: returnTypedElem,
							}
							fi.Returns = append(fi.Returns, ri)
						}
					} else {
						// unnamed return value
						returnTypedElem := parseTypedElement(result.Type, "", nil, ti.GenDecl, file, ti.Package, nil)
						ri := ReturnInfo{
							TypedElement: returnTypedElem,
						}
						fi.Returns = append(fi.Returns, ri)
					}
				}
			}
			ti.AddMethod(fi)
		}
	}
}

func (ti *TypeInfo) AddField(field FieldInfo) {
	ti.Fields = append(ti.Fields, field)
}

func (ti *TypeInfo) AddMethod(method FunctionInfo) {
	ti.Methods = append(ti.Methods, method)
}

func (ti *TypeInfo) ParseField(field *ast.Field) FieldInfo {
	fi := NewFieldInfoFromAst(field, ti.GenDecl, ti.File, ti.Package, nil)
	ti.AddField(fi)
	return fi
}

// parseFields parses the fields of a struct type with proper context for caching
func (ti *TypeInfo) parseFields(ctx *ProcessContext) {
	if ti.Kind == TypeKindStruct && ti.structType != nil && ti.structType.Fields != nil {
		for _, field := range ti.structType.Fields.List {
			fi := NewFieldInfoFromAst(field, ti.GenDecl, ti.File, ti.Package, ctx)
			ti.AddField(fi)
		}
		// Parse methods if we have file context and should scan struct methods
		if ti.File != nil && ti.Name != "" && shouldScanStructMethods(ctx) {
			ti.parseMethods(ti.File, ti.Name)
		}
		// Clear the structType reference as it's no longer needed
		ti.structType = nil
	} else if (ti.Kind == TypeKindArray || ti.Kind == TypeKindSlice || ti.Kind == TypeKindMap) && ti.structType != nil {
		// Parse container type information from TypeSpec
		ti.parseContainerTypes(ctx)
		ti.structType = nil
	} else if ti.Kind == TypeKindFunction && ti.structType != nil {
		// Parse function signature from TypeSpec
		ti.parseFunctionSignature(ctx)
		ti.structType = nil
	} else if ti.Kind == TypeKindInterface && ti.interfaceType != nil && ti.interfaceType.Methods != nil {
		// Parse interface methods
		for _, method := range ti.interfaceType.Methods.List {
			if len(method.Names) > 0 {
				// Method declaration
				for _, methodName := range method.Names {
					if funcType, ok := method.Type.(*ast.FuncType); ok {
						fi := FunctionInfo{
							Name:        methodName.Name,
							File:        ti.File,
							Comment:     utils.ExtractCommentText([]*ast.CommentGroup{method.Doc, method.Comment}),
							Annotations: annotations.ParseAnnotations([]*ast.CommentGroup{method.Doc, method.Comment}),
							Parms:       []ParameterInfo{},
							Returns:     []ReturnInfo{},
							Visibility:  determineVisibility(methodName.Name),
						}

						// parse parameters
						if funcType.Params != nil {
							for _, param := range funcType.Params.List {
								if len(param.Names) > 0 {
									for _, paramName := range param.Names {
										paramTypedElem := parseTypedElement(param.Type, paramName.Name, nil, ti.GenDecl, ti.File, ti.Package, ctx)
										pi := ParameterInfo{
											TypedElement: paramTypedElem,
										}
										fi.Parms = append(fi.Parms, pi)
									}
								} else {
									// unnamed parameter
									paramTypedElem := parseTypedElement(param.Type, "", nil, ti.GenDecl, ti.File, ti.Package, ctx)
									pi := ParameterInfo{
										TypedElement: paramTypedElem,
									}
									fi.Parms = append(fi.Parms, pi)
								}
							}
						}

						// parse return values
						if funcType.Results != nil {
							for _, result := range funcType.Results.List {
								if len(result.Names) > 0 {
									for _, returnName := range result.Names {
										returnTypedElem := parseTypedElement(result.Type, returnName.Name, nil, ti.GenDecl, ti.File, ti.Package, ctx)
										ri := ReturnInfo{
											TypedElement: returnTypedElem,
										}
										fi.Returns = append(fi.Returns, ri)
									}
								} else {
									// unnamed return value
									returnTypedElem := parseTypedElement(result.Type, "", nil, ti.GenDecl, ti.File, ti.Package, ctx)
									ri := ReturnInfo{
										TypedElement: returnTypedElem,
									}
									fi.Returns = append(fi.Returns, ri)
								}
							}
						}

						ti.AddMethod(fi)
					}
				}
			} else {
				// Embedded interface - could be handled here if needed
				// For now, we'll skip embedded interfaces
			}
		}
		// Clear the interfaceType reference as it's no longer needed
		ti.interfaceType = nil
	}
}

// parseTypeParams parses the type parameter constraints with proper context for caching
func (ti *TypeInfo) parseTypeParams(ctx *ProcessContext) {
	if ti.typeParamList != nil {
		paramIndex := 0
		for _, param := range ti.typeParamList.List {
			for range param.Names {
				if paramIndex < len(ti.TypeParams) {
					// Parse constraint if present
					if param.Type != nil {
						ti.TypeParams[paramIndex].Constraint = GetOrCreateTypeInfo(ctx, param.Type, "", nil, nil, ti.File, ti.Package, nil)
						if ti.TypeParams[paramIndex].Constraint != nil {
							ti.TypeParams[paramIndex].TypeRef = ti.TypeParams[paramIndex].Constraint.CannonicalName
						}
					}
				}
				paramIndex++
			}
		}
		// Clear the typeParamList reference as it's no longer needed
		ti.typeParamList = nil
	}
}

// parseAliasTarget parses the alias target with proper context for caching
func (ti *TypeInfo) parseAliasTarget(ctx *ProcessContext) {
	if ti.IsAlias && ti.TypeSpec != nil && ti.AliasTarget == nil {
		ti.AliasTarget = GetOrCreateTypeInfo(ctx, ti.TypeSpec.Type, "", nil, nil, ti.File, ti.Package, nil)
		if ti.AliasTarget != nil {
			ti.AliasTargetRef = ti.AliasTarget.CannonicalName
		}
	}
}

// parseGenericInstantiation parses the base type and type arguments for generic instantiation
func (ti *TypeInfo) parseGenericInstantiation(ctx *ProcessContext) {
	if !ti.IsGenericInstantiation || ti.TypeSpec == nil {
		return
	}

	switch t := ti.TypeSpec.Type.(type) {
	case *ast.IndexExpr:
		// Single type argument: Node[T]
		ti.BaseGenericType = GetOrCreateTypeInfo(ctx, t.X, "", nil, nil, ti.File, ti.Package, nil)
		if ti.BaseGenericType != nil {
			ti.BaseGenericTypeRef = ti.BaseGenericType.CannonicalName
		}
		argType := GetOrCreateTypeInfo(ctx, t.Index, "", nil, nil, ti.File, ti.Package, nil)
		ti.TypeArguments = []*TypeInfo{argType}
		if argType != nil {
			ti.TypeArgumentRefs = []string{argType.CannonicalName}
		}

	case *ast.IndexListExpr:
		// Multiple type arguments: Node[T, P]
		ti.BaseGenericType = GetOrCreateTypeInfo(ctx, t.X, "", nil, nil, ti.File, ti.Package, nil)
		if ti.BaseGenericType != nil {
			ti.BaseGenericTypeRef = ti.BaseGenericType.CannonicalName
		}
		ti.TypeArgumentRefs = []string{}
		for _, arg := range t.Indices {
			argType := GetOrCreateTypeInfo(ctx, arg, "", nil, nil, ti.File, ti.Package, nil)
			ti.TypeArguments = append(ti.TypeArguments, argType)
			if argType != nil {
				ti.TypeArgumentRefs = append(ti.TypeArgumentRefs, argType.CannonicalName)
			}
		}
	}

	// Update kind based on base type while preserving generic instantiation info
	if ti.BaseGenericType != nil {
		// Inherit the actual type structure (struct, interface, function, etc.)
		ti.Kind = ti.BaseGenericType.Kind

		// Inherit fields from the base generic type if available
		if len(ti.BaseGenericType.Fields) > 0 {
			ti.Fields = make([]FieldInfo, len(ti.BaseGenericType.Fields))
			copy(ti.Fields, ti.BaseGenericType.Fields)
		}

		// Inherit methods from the base generic type if available
		if len(ti.BaseGenericType.Methods) > 0 {
			ti.Methods = make([]FunctionInfo, len(ti.BaseGenericType.Methods))
			copy(ti.Methods, ti.BaseGenericType.Methods)
		}
	}
}

// parseContainerTypes parses element and key types for array/slice/map types
func (ti *TypeInfo) parseContainerTypes(ctx *ProcessContext) {
	if ti.TypeSpec == nil {
		return
	}

	switch t := ti.TypeSpec.Type.(type) {
	case *ast.ArrayType:
		// Parse element type for array or slice
		ti.ElementType = GetOrCreateTypeInfo(ctx, t.Elt, "", nil, nil, ti.File, ti.Package, nil)
		if ti.ElementType != nil {
			ti.ElementTypeRef = ti.ElementType.CannonicalName
		}
	case *ast.MapType:
		// Parse key and value types for map
		ti.KeyType = GetOrCreateTypeInfo(ctx, t.Key, "", nil, nil, ti.File, ti.Package, nil)
		if ti.KeyType != nil {
			ti.KeyTypeRef = ti.KeyType.CannonicalName
		}
		ti.ElementType = GetOrCreateTypeInfo(ctx, t.Value, "", nil, nil, ti.File, ti.Package, nil)
		if ti.ElementType != nil {
			ti.ElementTypeRef = ti.ElementType.CannonicalName
		}
	}
}

// parseFunctionSignature parses parameters and return types for function types
func (ti *TypeInfo) parseFunctionSignature(ctx *ProcessContext) {
	if ti.TypeSpec == nil {
		return
	}

	if funcType, ok := ti.TypeSpec.Type.(*ast.FuncType); ok {
		// Create a FunctionInfo to hold the signature
		fi := &FunctionInfo{
			Name:        ti.Name,
			File:        ti.File,
			Comment:     ti.Comment,
			Annotations: ti.Annotations,
			Parms:       []ParameterInfo{},
			Returns:     []ReturnInfo{},
			Visibility:  determineVisibility(ti.Name),
		}

		// Parse parameters
		if funcType.Params != nil {
			for _, param := range funcType.Params.List {
				if len(param.Names) > 0 {
					// named parameters
					for _, paramName := range param.Names {
						paramTypedElem := parseTypedElement(param.Type, paramName.Name, nil, ti.GenDecl, ti.File, ti.Package, ctx)
						pi := ParameterInfo{
							TypedElement: paramTypedElem,
						}
						fi.Parms = append(fi.Parms, pi)
					}
				} else {
					// unnamed parameter
					paramTypedElem := parseTypedElement(param.Type, "", nil, ti.GenDecl, ti.File, ti.Package, ctx)
					pi := ParameterInfo{
						TypedElement: paramTypedElem,
					}
					fi.Parms = append(fi.Parms, pi)
				}
			}
		}

		// Parse return values
		if funcType.Results != nil {
			for _, result := range funcType.Results.List {
				if len(result.Names) > 0 {
					// named return values
					for _, returnName := range result.Names {
						returnTypedElem := parseTypedElement(result.Type, returnName.Name, nil, ti.GenDecl, ti.File, ti.Package, ctx)
						ri := ReturnInfo{
							TypedElement: returnTypedElem,
						}
						fi.Returns = append(fi.Returns, ri)
					}
				} else {
					// unnamed return value
					returnTypedElem := parseTypedElement(result.Type, "", nil, ti.GenDecl, ti.File, ti.Package, ctx)
					ri := ReturnInfo{
						TypedElement: returnTypedElem,
					}
					fi.Returns = append(fi.Returns, ri)
				}
			}
		}

		// Assign the function signature
		ti.FunctionSig = fi
	}
}

// parseStructTags parses Go struct tags from a tag string
func parseStructTags(tagString string) annotations.StructTags {
	tags := make(annotations.StructTags)

	// Remove the backticks if present
	if len(tagString) >= 2 && tagString[0] == '`' && tagString[len(tagString)-1] == '`' {
		tagString = tagString[1 : len(tagString)-1]
	}

	// Parse the tag string
	for tagString != "" {
		// Skip leading space
		i := 0
		for i < len(tagString) && tagString[i] == ' ' {
			i++
		}
		tagString = tagString[i:]
		if tagString == "" {
			break
		}

		// Scan to colon to find key
		i = 0
		for i < len(tagString) && tagString[i] != ':' {
			i++
		}
		if i >= len(tagString) {
			break
		}
		key := tagString[:i]
		tagString = tagString[i+1:]

		// Skip leading space
		i = 0
		for i < len(tagString) && tagString[i] == ' ' {
			i++
		}
		tagString = tagString[i:]
		if tagString == "" {
			break
		}

		// Scan quoted string to find value
		if tagString[0] != '"' {
			break
		}
		i = 1
		for i < len(tagString) && tagString[i] != '"' {
			if tagString[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tagString) {
			break
		}
		value := tagString[1:i]
		tagString = tagString[i+1:]

		// Unescape value if needed
		value = strings.ReplaceAll(value, `\"`, `"`)
		value = strings.ReplaceAll(value, `\\`, `\`)

		tags[key] = value
	}

	return tags
}

// shouldScanStructMethods determines if struct methods should be scanned based on ScanOptions
func shouldScanStructMethods(ctx *ProcessContext) bool {
	if ctx == nil || ctx.Config == nil {
		return true // Default to scanning everything if no config
	}

	scanOptions := &ctx.Config.Scanning.ScanOptions
	return scanOptions.StructMethods != config.ScanModeNone && scanOptions.StructMethods != config.ScanModeDisabled
}
